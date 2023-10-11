package datasync

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"go.temporal.io/sdk/activity"
	"golang.org/x/sync/errgroup"

	_ "github.com/benthosdev/benthos/v4/public/components/aws"
	_ "github.com/benthosdev/benthos/v4/public/components/io"
	_ "github.com/benthosdev/benthos/v4/public/components/pure"
	_ "github.com/benthosdev/benthos/v4/public/components/sql"
	"github.com/benthosdev/benthos/v4/public/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	mgmtv1alpha1 "github.com/nucleuscloud/neosync/backend/gen/go/protos/mgmt/v1alpha1"
	"github.com/nucleuscloud/neosync/backend/gen/go/protos/mgmt/v1alpha1/mgmtv1alpha1connect"
	neosync_benthos "github.com/nucleuscloud/neosync/worker/internal/benthos"
	_ "github.com/nucleuscloud/neosync/worker/internal/benthos/plugins"
	dbschemas_postgres "github.com/nucleuscloud/neosync/worker/internal/dbschemas/postgres"
)

type GenerateBenthosConfigsRequest struct {
	JobId      string
	BackendUrl string
	WorkflowId string
}
type GenerateBenthosConfigsResponse struct {
	BenthosConfigs []*benthosConfigResponse
}

type benthosConfigResponse struct {
	Name      string
	DependsOn []string
	Config    *neosync_benthos.BenthosConfig
}

type Activities struct{}

func (a *Activities) GenerateBenthosConfigs(
	ctx context.Context,
	req *GenerateBenthosConfigsRequest,
) (*GenerateBenthosConfigsResponse, error) {
	logger := activity.GetLogger(ctx)
	_ = logger
	go func() {
		for {
			select {
			case <-time.After(1 * time.Second):
				activity.RecordHeartbeat(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()

	pgpoolmap := map[string]*pgxpool.Pool{}

	job, err := a.getJobById(ctx, req.BackendUrl, req.JobId)
	if err != nil {
		return nil, err
	}
	responses := []*benthosConfigResponse{}

	sourceConnection, err := a.getConnectionById(ctx, req.BackendUrl, job.Source.ConnectionId)
	if err != nil {
		return nil, err
	}

	switch connection := sourceConnection.ConnectionConfig.Config.(type) {
	case *mgmtv1alpha1.ConnectionConfig_PgConfig:
		dsn, err := getPgDsn(connection.PgConfig)
		if err != nil {
			return nil, err
		}

		groupedMappings := groupMappingsByTable(job.Mappings)
		for key, mappings := range groupedMappings {
			cols := buildPlainColumns(mappings)
			if len(cols) == 0 {
				// skipping table as no columns are mapped
				continue
			}
			bc := &neosync_benthos.BenthosConfig{
				StreamConfig: neosync_benthos.StreamConfig{
					Input: &neosync_benthos.InputConfig{
						Inputs: neosync_benthos.Inputs{
							SqlSelect: &neosync_benthos.SqlSelect{
								Driver: "postgres",
								Dsn:    dsn,

								Table:   key,
								Columns: buildPlainColumns(mappings),
							},
						},
					},
					Pipeline: &neosync_benthos.PipelineConfig{
						Threads:    -1,
						Processors: []neosync_benthos.ProcessorConfig{},
					},
					Output: &neosync_benthos.OutputConfig{
						Outputs: neosync_benthos.Outputs{
							Broker: &neosync_benthos.OutputBrokerConfig{
								Pattern: "fan_out",
								Outputs: []neosync_benthos.Outputs{},
							},
						},

						// Broker: &neosync_benthos.OutputBrokerConfig{
						// 	Pattern: "fan_out",
						// 	Outputs: []neosync_benthos.Outputs{},
						// },
					},
				},
			}
			mutation, err := buildProcessorMutation(job.Mappings)
			if err != nil {
				return nil, err
			}
			if mutation != "" {
				bc.StreamConfig.Pipeline.Processors = append(bc.StreamConfig.Pipeline.Processors, neosync_benthos.ProcessorConfig{
					Mutation: mutation,
				})
			}
			responses = append(responses, &benthosConfigResponse{
				Name:      key, // todo: may need to expand on this
				Config:    bc,
				DependsOn: []string{},
			})
		}
		if _, ok := pgpoolmap[dsn]; !ok {
			pool, err := pgxpool.New(ctx, dsn)
			if err != nil {
				return nil, err
			}
			defer pool.Close()
			pgpoolmap[dsn] = pool
		}
		pool := pgpoolmap[dsn]

		// validate job mappings align with sql connections
		dbschemas, err := dbschemas_postgres.GetDatabaseSchemas(ctx, pool)
		if err != nil {
			return nil, err
		}
		if !areMappingsSubsetOfSchemas(dbschemas, job.Mappings) {
			return nil, errors.New("job mappings are not equal to or a subset of the database schema found in the source connection")
		}
		sqlOpts := job.Source.Options.GetSqlOptions()
		if sqlOpts != nil &&
			sqlOpts.HaltOnNewColumnAddition != nil &&
			*sqlOpts.HaltOnNewColumnAddition &&
			shouldHaltOnSchemaAddition(dbschemas, job.Mappings) {
			msg := "job mappings does not contain a column mapping for all " +
				"columns found in the source connection for the selected schemas and tables"
			return nil, errors.New(msg)
		}

		allConstraints, err := a.getAllFkConstraintsFromMappings(ctx, pool, job.Mappings)
		if err != nil {
			return nil, err
		}
		td := dbschemas_postgres.GetPostgresTableDependencies(allConstraints)

		for _, resp := range responses {
			dependsOn, ok := td[resp.Name]
			if ok {
				resp.DependsOn = dependsOn
			}
		}

	default:
		return nil, fmt.Errorf("unsupported source connection")
	}

	for _, destination := range job.Destinations {
		destinationConnection, err := a.getConnectionById(ctx, req.BackendUrl, destination.ConnectionId)
		if err != nil {
			return nil, err
		}
		for _, resp := range responses {
			switch connection := destinationConnection.ConnectionConfig.Config.(type) {
			case *mgmtv1alpha1.ConnectionConfig_PgConfig:
				dsn, err := getPgDsn(connection.PgConfig)
				if err != nil {
					return nil, err
				}

				truncateBeforeInsert := false
				initSchema := true
				sqlOpts := destination.Options.GetSqlOptions()
				if sqlOpts != nil && sqlOpts.InitDbSchema != nil {
					initSchema = *sqlOpts.InitDbSchema
				}

				if sqlOpts != nil && sqlOpts.TruncateBeforeInsert != nil {
					truncateBeforeInsert = *sqlOpts.TruncateBeforeInsert
				}

				pool := pgpoolmap[resp.Config.Input.SqlSelect.Dsn]
				// todo: make this more efficient to reduce amount of times we have to connect to the source database
				schema, table := splitTableKey(resp.Config.Input.SqlSelect.Table)
				initStmt, err := a.getInitStatementFromPostgres(
					ctx,
					pool,
					schema,
					table,
					&initStatementOpts{
						TruncateBeforeInsert: truncateBeforeInsert,
						InitSchema:           initSchema,
					},
				)
				if err != nil {
					return nil, err
				}
				resp.Config.Output.Broker.Outputs = append(resp.Config.Output.Broker.Outputs, neosync_benthos.Outputs{
					SqlInsert: &neosync_benthos.SqlInsert{
						Driver: "postgres",
						Dsn:    dsn,

						Table:         resp.Config.Input.SqlSelect.Table,
						Columns:       resp.Config.Input.SqlSelect.Columns,
						ArgsMapping:   buildPlainInsertArgs(resp.Config.Input.SqlSelect.Columns),
						InitStatement: initStmt,
					},
					// Fallback: []neosync_benthos.Outputs{
					// 	{
					// 		DropOn: &neosync_benthos.DropOnConfig{
					// 			Error: true,
					// 			Output: neosync_benthos.Outputs{
					// 				SqlInsert: &neosync_benthos.SqlInsert{
					// 					Driver: "postgres",
					// 					Dsn:    dsn,

					// 					Table:         resp.Config.Input.SqlSelect.Table,
					// 					Columns:       resp.Config.Input.SqlSelect.Columns,
					// 					ArgsMapping:   buildPlainInsertArgs(resp.Config.Input.SqlSelect.Columns),
					// 					InitStatement: initStmt,
					// 				},
					// 			},
					// 		},
					// 	},
					// },
					// Retry: &neosync_benthos.RetryConfig{
					// 	InlineRetryConfig: neosync_benthos.InlineRetryConfig{
					// 		MaxRetries: 1,
					// 	},
					// 	Output: neosync_benthos.OutputConfig{
					// 		Outputs: neosync_benthos.Outputs{
					// 			SqlInsert: &neosync_benthos.SqlInsert{
					// 				Driver: "postgres",
					// 				Dsn:    dsn,

					// 				Table:         resp.Config.Input.SqlSelect.Table,
					// 				Columns:       resp.Config.Input.SqlSelect.Columns,
					// 				ArgsMapping:   buildPlainInsertArgs(resp.Config.Input.SqlSelect.Columns),
					// 				InitStatement: initStmt,
					// 			},
					// 		},
					// 	},
					// },
				})

			case *mgmtv1alpha1.ConnectionConfig_AwsS3Config:
				s3pathpieces := []string{}
				if connection.AwsS3Config.PathPrefix != nil && *connection.AwsS3Config.PathPrefix != "" {
					s3pathpieces = append(s3pathpieces, strings.Trim(*connection.AwsS3Config.PathPrefix, "/"))
				}
				s3pathpieces = append(
					s3pathpieces,
					"workflows",
					req.WorkflowId,
					"activities",
					resp.Name, // may need to do more here
					"data",
					`${!count("files")}.json.gz}`,
				)

				resp.Config.Output.Broker.Outputs = append(resp.Config.Output.Broker.Outputs, neosync_benthos.Outputs{
					Retry: &neosync_benthos.RetryConfig{
						InlineRetryConfig: neosync_benthos.InlineRetryConfig{
							MaxRetries: 1,
						},
						Output: neosync_benthos.OutputConfig{
							Outputs: neosync_benthos.Outputs{
								AwsS3: &neosync_benthos.AwsS3Insert{
									Bucket:      connection.AwsS3Config.BucketArn,
									MaxInFlight: 64,
									Path:        fmt.Sprintf("/%s", strings.Join(s3pathpieces, "/")),
									Batching: &neosync_benthos.Batching{
										Count:  100,
										Period: "5s",
										Processors: []*neosync_benthos.BatchProcessor{
											{Archive: &neosync_benthos.ArchiveProcessor{Format: "json_array"}},
											{Compress: &neosync_benthos.CompressProcessor{Algorithm: "gzip"}},
										},
									},
								},
							},
						},
					},
				})
				// todo: configure provided aws creds
			default:
				return nil, fmt.Errorf("unsupported destination connection config")
			}
		}
	}

	return &GenerateBenthosConfigsResponse{
		BenthosConfigs: responses,
	}, nil
}

func areMappingsSubsetOfSchemas(
	dbschemas []*dbschemas_postgres.DatabaseSchema,
	mappings []*mgmtv1alpha1.JobMapping,
) bool {
	tableColMappings := getUniqueColMappingsMap(mappings)
	groupedSchemas := getUniqueSchemaColMappings(dbschemas)

	for key := range groupedSchemas {
		// For this method, we only care about the schemas+tables that we currently have mappings for
		if _, ok := tableColMappings[key]; !ok {
			delete(groupedSchemas, key)
		}
	}

	if len(tableColMappings) != len(groupedSchemas) {
		return false
	}

	// tests to make sure that every column in the col mappings is present in the db schema
	for table, cols := range tableColMappings {
		schemaCols, ok := groupedSchemas[table]
		if !ok {
			return false
		}
		// job mappings has more columns than the schema
		if len(cols) > len(schemaCols) {
			return false
		}
		for col := range cols {
			if _, ok := schemaCols[col]; !ok {
				return false
			}
		}
	}
	return true
}

func getUniqueSchemaColMappings(
	dbschemas []*dbschemas_postgres.DatabaseSchema,
) map[string]map[string]struct{} {
	groupedSchemas := map[string]map[string]struct{}{} // ex: {public.users: { id: struct{}{}, created_at: struct{}{}}}
	for _, record := range dbschemas {
		key := buildBenthosTable(record.TableSchema, record.TableName)
		if _, ok := groupedSchemas[key]; ok {
			groupedSchemas[key][record.ColumnName] = struct{}{}
		} else {
			groupedSchemas[key] = map[string]struct{}{
				record.ColumnName: {},
			}
		}
	}
	return groupedSchemas
}

func getUniqueColMappingsMap(
	mappings []*mgmtv1alpha1.JobMapping,
) map[string]map[string]struct{} {
	tableColMappings := map[string]map[string]struct{}{}
	for _, mapping := range mappings {
		key := buildBenthosTable(mapping.Schema, mapping.Table)
		if _, ok := tableColMappings[key]; ok {
			tableColMappings[key][mapping.Column] = struct{}{}
		} else {
			tableColMappings[key] = map[string]struct{}{
				mapping.Column: {},
			}
		}
	}
	return tableColMappings
}

func shouldHaltOnSchemaAddition(
	dbschemas []*dbschemas_postgres.DatabaseSchema,
	mappings []*mgmtv1alpha1.JobMapping,
) bool {
	tableColMappings := getUniqueColMappingsMap(mappings)
	groupedSchemas := getUniqueSchemaColMappings(dbschemas)

	if len(tableColMappings) != len(groupedSchemas) {
		return true
	}

	for table, cols := range groupedSchemas {
		mappingCols, ok := tableColMappings[table]
		if !ok {
			return true
		}
		if len(cols) > len(mappingCols) {
			return true
		}
		for col := range cols {
			if _, ok := mappingCols[col]; !ok {
				return true
			}
		}
	}
	return false
}

func (a *Activities) getAllFkConstraintsFromMappings(
	ctx context.Context,
	conn DBTX,
	mappings []*mgmtv1alpha1.JobMapping,
) ([]*dbschemas_postgres.ForeignKeyConstraint, error) {
	uniqueSchemas := getUniqueSchemasFromMappings(mappings)
	holder := make([][]*dbschemas_postgres.ForeignKeyConstraint, len(uniqueSchemas))
	errgrp, errctx := errgroup.WithContext(ctx)
	for idx := range uniqueSchemas {
		idx := idx
		schema := uniqueSchemas[idx]
		errgrp.Go(func() error {
			constraints, err := dbschemas_postgres.GetForeignKeyConstraints(errctx, conn, schema)
			if err != nil {
				return err
			}
			holder[idx] = constraints
			return nil
		})
	}

	if err := errgrp.Wait(); err != nil {
		return nil, err
	}

	output := []*dbschemas_postgres.ForeignKeyConstraint{}
	for _, schemas := range holder {
		output = append(output, schemas...)
	}
	return output, nil
}

type initStatementOpts struct {
	TruncateBeforeInsert bool
	InitSchema           bool
}

type DBTX interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (a *Activities) getInitStatementFromPostgres(
	ctx context.Context,
	conn DBTX,
	schema string,
	table string,
	opts *initStatementOpts,
) (string, error) {

	statements := []string{}
	if opts != nil && opts.InitSchema {
		stmt, err := dbschemas_postgres.GetTableCreateStatement(ctx, conn, &dbschemas_postgres.GetTableCreateStatementRequest{
			Schema: schema,
			Table:  table,
		})
		if err != nil {
			return "", err
		}
		statements = append(statements, stmt)
	}
	if opts != nil && opts.TruncateBeforeInsert {
		statements = append(statements, fmt.Sprintf("TRUNCATE TABLE %s.%s CASCADE;", schema, table))
	}
	return strings.Join(statements, "\n"), nil
}

func buildPlainColumns(mappings []*mgmtv1alpha1.JobMapping) []string {
	columns := []string{}

	for _, col := range mappings {
		if !col.Exclude {
			columns = append(columns, col.Column)
		}
	}

	return columns
}

func splitTableKey(key string) (schema, table string) {
	pieces := strings.Split(key, ".")
	if len(pieces) == 1 {
		return "public", pieces[0]
	}
	return pieces[0], pieces[1]
}

// used to record metadata in activity event history
type SyncMetadata struct {
	Schema string
	Table  string
}
type SyncRequest struct {
	BenthosConfig string
}
type SyncResponse struct{}

func (a *Activities) Sync(ctx context.Context, req *SyncRequest, metadata *SyncMetadata) (*SyncResponse, error) {
	logger := activity.GetLogger(ctx)
	var benthosStream *service.Stream
	go func() {
		for {
			select {
			case <-time.After(1 * time.Second):
				activity.RecordHeartbeat(ctx)
			case <-ctx.Done():
				if benthosStream != nil {
					// this must be here because stream.Run(ctx) doesn't seem to fully obey a canceled context when
					// a sink is in an error state. We want to explicitly call stop here because the workflow has been canceled.
					err := benthosStream.Stop(ctx)
					if err != nil {
						logger.Error(err.Error())
					}
				}
				return
			}
		}
	}()

	streambldr := service.NewStreamBuilder()

	err := streambldr.SetYAML(req.BenthosConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to convert benthos config to yaml for stream builder: %w", err)
	}

	stream, err := streambldr.Build()
	if err != nil {
		return nil, fmt.Errorf("unable to build benthos stream: %w", err)
	}
	benthosStream = stream

	err = stream.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to run benthos stream: %w", err)
	}
	return &SyncResponse{}, nil
}

func groupMappingsByTable(
	mappings []*mgmtv1alpha1.JobMapping,
) map[string][]*mgmtv1alpha1.JobMapping {
	output := map[string][]*mgmtv1alpha1.JobMapping{}

	for _, mapping := range mappings {
		key := buildBenthosTable(mapping.Schema, mapping.Table)
		output[key] = append(output[key], mapping)
	}
	return output
}

func getUniqueSchemasFromMappings(mappings []*mgmtv1alpha1.JobMapping) []string {
	schemas := map[string]struct{}{}
	for _, mapping := range mappings {
		schemas[mapping.Schema] = struct{}{}
	}

	output := make([]string, 0, len(schemas))

	for schema := range schemas {
		output = append(output, schema)
	}
	return output
}

func buildBenthosTable(schema, table string) string {
	if schema != "" {
		return fmt.Sprintf("%s.%s", schema, table)
	}
	return table
}

func (a *Activities) getJobById(ctx context.Context, backendurl, jobId string) (*mgmtv1alpha1.Job, error) {
	jobclient := mgmtv1alpha1connect.NewJobServiceClient(
		http.DefaultClient,
		backendurl,
	)

	getjobResp, err := jobclient.GetJob(ctx, connect.NewRequest(&mgmtv1alpha1.GetJobRequest{
		Id: jobId,
	}))
	if err != nil {
		return nil, err
	}

	return getjobResp.Msg.Job, nil
}

func (a *Activities) getConnectionById(ctx context.Context, backendurl, connectionId string) (*mgmtv1alpha1.Connection, error) {
	connclient := mgmtv1alpha1connect.NewConnectionServiceClient(
		http.DefaultClient,
		backendurl,
	)

	getConnResp, err := connclient.GetConnection(ctx, connect.NewRequest(&mgmtv1alpha1.GetConnectionRequest{
		Id: connectionId,
	}))
	if err != nil {
		return nil, err
	}
	return getConnResp.Msg.Connection, nil
}

func getPgDsn(
	config *mgmtv1alpha1.PostgresConnectionConfig,
) (string, error) {
	switch cfg := config.ConnectionConfig.(type) {
	case *mgmtv1alpha1.PostgresConnectionConfig_Connection:
		dburl := fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s",
			cfg.Connection.User,
			cfg.Connection.Pass,
			cfg.Connection.Host,
			cfg.Connection.Port,
			cfg.Connection.Name,
		)
		if cfg.Connection.SslMode != nil && *cfg.Connection.SslMode != "" {
			dburl = fmt.Sprintf("%s?sslmode=%s", dburl, *cfg.Connection.SslMode)
		}
		return dburl, nil
	case *mgmtv1alpha1.PostgresConnectionConfig_Url:
		return cfg.Url, nil
	default:
		return "", fmt.Errorf("unsupported postgres connection config type")
	}
}

func buildProcessorMutation(cols []*mgmtv1alpha1.JobMapping) (string, error) {
	pieces := []string{}

	for _, col := range cols {
		if col.Transformer != "" && col.Transformer != "passthrough" {
			mutation, err := computeMutationFunction(col.Transformer)
			if err != nil {
				return "", fmt.Errorf("%s is not a supported transformation: %w", col.Transformer, err)
			}
			pieces = append(pieces, fmt.Sprintf("root.%s = %s", col.Column, mutation))
		}
	}
	return strings.Join(pieces, "\n"), nil
}

func buildPlainInsertArgs(cols []string) string {
	if len(cols) == 0 {
		return ""
	}
	pieces := make([]string, len(cols))
	for idx, col := range cols {
		pieces[idx] = fmt.Sprintf("this.%s", col)
	}
	return fmt.Sprintf("root = [%s]", strings.Join(pieces, ", "))
}

func computeMutationFunction(transformer string) (string, error) {
	switch transformer {
	case "uuid_v4":
		return "uuid_v4()", nil
	case "latitude":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "longitude":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "unix_time":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "date":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "time_string":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "month_name":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "year_string":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "day_of_week":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "day_of_month":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "timestamp":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "century":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "timezone":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "time_period":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "email":
		return fmt.Sprintf("this.%s.emailtransformer(true, true)", transformer), nil
	case "mac_address":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "domain_name":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "url":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "username":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "ipv4":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "ipv6":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "password":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "jwt":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "word":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "sentence":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "paragraph":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "cc_type":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "cc_number":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "currency":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "amount_with_currency":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "title_male":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "title_female":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "first_name":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "first_name_male":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "first_name_female":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "last_name":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "name":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "gender":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "chinese_first_name":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "chinese_last_name":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "chinese_name":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "phone_number":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "toll_free_phone_number":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "e164_phone_number":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "uuid_hyphenated":
		return fmt.Sprintf("fake(%q)", transformer), nil
	case "uuid_digit":
		return fmt.Sprintf("fake(%q)", transformer), nil
	default:
		return "", fmt.Errorf("unsupported transformer")
	}
}
