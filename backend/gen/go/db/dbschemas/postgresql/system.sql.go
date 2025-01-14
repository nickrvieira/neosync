// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0
// source: system.sql

package pg_queries

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const getCustomFunctionsBySchemaAndTables = `-- name: GetCustomFunctionsBySchemaAndTables :many
WITH relevant_schemas_tables AS (
    SELECT c.oid, n.nspname AS schema_name, c.relname AS table_name
    FROM pg_catalog.pg_class c
    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = $1
    AND c.relname = ANY($2::TEXT[])
),
trigger_functions AS (
    SELECT DISTINCT
        n.nspname AS schema_name,
        p.proname AS function_name,
        pg_catalog.pg_get_functiondef(p.oid) AS definition,
        pg_catalog.pg_get_function_identity_arguments(p.oid) AS function_signature
    FROM pg_catalog.pg_trigger t
    JOIN pg_catalog.pg_proc p ON t.tgfoid = p.oid
    JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
    WHERE t.tgrelid IN (SELECT oid FROM relevant_schemas_tables)
    AND NOT t.tgisinternal
),
column_default_functions AS (
    SELECT DISTINCT
        n.nspname AS schema_name,
        p.proname AS function_name,
        pg_catalog.pg_get_functiondef(p.oid) AS definition,
        pg_catalog.pg_get_function_identity_arguments(p.oid) AS function_signature
    FROM pg_catalog.pg_attrdef ad
    JOIN pg_catalog.pg_depend d ON ad.oid = d.objid
    JOIN pg_catalog.pg_proc p ON d.refobjid = p.oid
    JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
    WHERE ad.adrelid IN (SELECT oid FROM relevant_schemas_tables)
    AND d.refclassid = 'pg_proc'::regclass
    AND d.classid = 'pg_attrdef'::regclass
)
SELECT
    schema_name,
    function_name,
    function_signature,
    definition
FROM
    trigger_functions
UNION
SELECT
    schema_name,
    function_name,
    function_signature,
    definition
FROM
    column_default_functions
ORDER BY
    schema_name,
    function_name
`

type GetCustomFunctionsBySchemaAndTablesParams struct {
	Schema string
	Tables []string
}

type GetCustomFunctionsBySchemaAndTablesRow struct {
	SchemaName        string
	FunctionName      string
	FunctionSignature string
	Definition        string
}

func (q *Queries) GetCustomFunctionsBySchemaAndTables(ctx context.Context, db DBTX, arg *GetCustomFunctionsBySchemaAndTablesParams) ([]*GetCustomFunctionsBySchemaAndTablesRow, error) {
	rows, err := db.Query(ctx, getCustomFunctionsBySchemaAndTables, arg.Schema, arg.Tables)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetCustomFunctionsBySchemaAndTablesRow
	for rows.Next() {
		var i GetCustomFunctionsBySchemaAndTablesRow
		if err := rows.Scan(
			&i.SchemaName,
			&i.FunctionName,
			&i.FunctionSignature,
			&i.Definition,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getCustomSequencesBySchemaAndTables = `-- name: GetCustomSequencesBySchemaAndTables :many
WITH relevant_schemas_tables AS (
    SELECT c.oid, n.nspname AS schema_name, c.relname AS table_name
    FROM pg_catalog.pg_class c
    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = $1
    AND c.relname = ANY($2::TEXT[])
),
all_sequences AS (
    SELECT
        seq.relname AS sequence_name,
        nsp.nspname AS schema_name,
        seq.oid AS sequence_oid
    FROM
        pg_catalog.pg_class seq
    JOIN
        pg_catalog.pg_namespace nsp ON seq.relnamespace = nsp.oid
    WHERE
        seq.relkind = 'S'
),
linked_to_serial AS (
    SELECT
        seq.relname AS sequence_name,
        nsp.nspname AS schema_name,
        seq.oid AS sequence_oid
    FROM
        pg_catalog.pg_class seq
    JOIN
        pg_catalog.pg_namespace nsp ON seq.relnamespace = nsp.oid
    JOIN
        pg_catalog.pg_depend dep ON dep.objid = seq.oid AND dep.classid = 'pg_catalog.pg_class'::regclass
    JOIN
        pg_catalog.pg_attrdef ad ON dep.refobjid = ad.adrelid AND dep.refobjsubid = ad.adnum
    WHERE
        pg_catalog.pg_get_expr(ad.adbin, ad.adrelid) LIKE 'nextval%'
),
custom_sequences AS (
    SELECT
        seq.sequence_name,
        seq.schema_name,
        seq.sequence_oid
    FROM
        all_sequences seq
    LEFT JOIN
        linked_to_serial serial ON seq.sequence_oid = serial.sequence_oid
    WHERE
        serial.sequence_oid IS NULL
)
SELECT DISTINCT
    cs.schema_name,
    cs.sequence_name,
    (
        'CREATE SEQUENCE ' || cs.schema_name || '.' || cs.sequence_name ||
        ' START WITH ' || seqs.start_value ||
        ' INCREMENT BY ' || seqs.increment_by ||
        ' MINVALUE ' || seqs.min_value ||
        ' MAXVALUE ' || seqs.max_value ||
        ' CACHE ' || seqs.cache_size ||
        CASE WHEN seqs.cycle THEN ' CYCLE' ELSE ' NO CYCLE' END || ';'
    )::text AS "definition"
FROM
    custom_sequences cs
JOIN
    relevant_schemas_tables rst ON cs.schema_name = rst.schema_name
JOIN
    pg_catalog.pg_sequences seqs ON seqs.schemaname = cs.schema_name AND seqs.sequencename = cs.sequence_name
ORDER BY
    cs.schema_name,
    cs.sequence_name
`

type GetCustomSequencesBySchemaAndTablesParams struct {
	Schema string
	Tables []string
}

type GetCustomSequencesBySchemaAndTablesRow struct {
	SchemaName   string
	SequenceName string
	Definition   string
}

func (q *Queries) GetCustomSequencesBySchemaAndTables(ctx context.Context, db DBTX, arg *GetCustomSequencesBySchemaAndTablesParams) ([]*GetCustomSequencesBySchemaAndTablesRow, error) {
	rows, err := db.Query(ctx, getCustomSequencesBySchemaAndTables, arg.Schema, arg.Tables)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetCustomSequencesBySchemaAndTablesRow
	for rows.Next() {
		var i GetCustomSequencesBySchemaAndTablesRow
		if err := rows.Scan(&i.SchemaName, &i.SequenceName, &i.Definition); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getCustomTriggersBySchemaAndTables = `-- name: GetCustomTriggersBySchemaAndTables :many
SELECT
    n.nspname AS schema_name,
    c.relname AS table_name,
    t.tgname AS trigger_name,
    pg_catalog.pg_get_triggerdef(t.oid, true) AS definition
FROM pg_catalog.pg_trigger t
JOIN pg_catalog.pg_class c ON c.oid = t.tgrelid
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE  (n.nspname || '.' || c.relname) = ANY($1::TEXT[])
AND NOT t.tgisinternal
ORDER BY
    schema_name,
    table_name,
    trigger_name
`

type GetCustomTriggersBySchemaAndTablesRow struct {
	SchemaName  string
	TableName   string
	TriggerName string
	Definition  string
}

func (q *Queries) GetCustomTriggersBySchemaAndTables(ctx context.Context, db DBTX, schematables []string) ([]*GetCustomTriggersBySchemaAndTablesRow, error) {
	rows, err := db.Query(ctx, getCustomTriggersBySchemaAndTables, schematables)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetCustomTriggersBySchemaAndTablesRow
	for rows.Next() {
		var i GetCustomTriggersBySchemaAndTablesRow
		if err := rows.Scan(
			&i.SchemaName,
			&i.TableName,
			&i.TriggerName,
			&i.Definition,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getDataTypesBySchemaAndTables = `-- name: GetDataTypesBySchemaAndTables :many
WITH custom_types AS (
    SELECT
        n.nspname AS schema_name,
        t.typname AS type_name,
        t.oid AS type_oid,
        CASE
            WHEN t.typtype = 'd' THEN 'domain'
            WHEN t.typtype = 'e' THEN 'enum'
            WHEN t.typtype = 'c' THEN 'composite'
        END AS type
    FROM
        pg_catalog.pg_type t
    JOIN
        pg_catalog.pg_namespace n ON n.oid = t.typnamespace
    WHERE
        n.nspname = $1
        AND t.typtype IN ('d', 'e', 'c')
),
table_columns AS (
    SELECT
        c.oid AS table_oid,
        a.atttypid AS type_oid
    FROM
        pg_catalog.pg_class c
    JOIN
        pg_catalog.pg_namespace n ON n.oid = c.relnamespace
    JOIN
        pg_catalog.pg_attribute a ON a.attrelid = c.oid
    WHERE
        n.nspname = $1
        AND c.relname = ANY($2::TEXT[])
        AND a.attnum > 0
        AND NOT a.attisdropped
),
relevant_custom_types AS (
    SELECT DISTINCT
        ct.schema_name,
        ct.type_name,
        ct.type_oid,
        ct.type
    FROM
        custom_types ct
    JOIN
        table_columns tc ON ct.type_oid = tc.type_oid
),
domain_defs AS (
    SELECT
        rct.schema_name,
        rct.type_name,
        rct.type,
        'CREATE DOMAIN ' || rct.schema_name || '.' || rct.type_name || ' AS ' ||
        pg_catalog.format_type(t.typbasetype, t.typtypmod) ||
        CASE
            WHEN t.typnotnull THEN ' NOT NULL' ELSE ''
        END || ' ' ||
        COALESCE('CONSTRAINT ' || conname || ' ' || pg_catalog.pg_get_constraintdef(c.oid), '') || ';' AS definition
    FROM
        relevant_custom_types rct
    JOIN
        pg_catalog.pg_type t ON rct.type_oid = t.oid
    LEFT JOIN
        pg_catalog.pg_constraint c ON t.oid = c.contypid
    WHERE
        rct.type = 'domain'
),
enum_defs AS (
    SELECT
        rct.schema_name,
        rct.type_name,
        rct.type,
        'CREATE TYPE ' || rct.schema_name || '.' || rct.type_name || ' AS ENUM (' ||
        string_agg('''' || e.enumlabel || '''', ', ') || ');' AS definition
    FROM
        relevant_custom_types rct
    JOIN
        pg_catalog.pg_type t ON rct.type_oid = t.oid
    JOIN
        pg_catalog.pg_enum e ON t.oid = e.enumtypid
    WHERE
        rct.type = 'enum'
    GROUP BY
        rct.schema_name, rct.type_name, rct.type
),
composite_defs AS (
    SELECT
        rct.schema_name,
        rct.type_name,
        rct.type,
        'CREATE TYPE ' || rct.schema_name || '.' || rct.type_name || ' AS (' ||
        string_agg(a.attname || ' ' || pg_catalog.format_type(a.atttypid, a.atttypmod), ', ') || ');' AS definition
    FROM
        relevant_custom_types rct
    JOIN
        pg_catalog.pg_type t ON rct.type_oid = t.oid
    JOIN
        pg_catalog.pg_class c ON c.oid = t.typrelid
    JOIN
        pg_catalog.pg_attribute a ON a.attrelid = c.oid
    WHERE
        rct.type = 'composite'
        AND a.attnum > 0
        AND NOT a.attisdropped
    GROUP BY
        rct.schema_name, rct.type_name, rct.type
)
SELECT
    schema_name,
    type_name,
    "type"::text,
    "definition"::text
FROM
    domain_defs
UNION ALL
SELECT
    schema_name,
    type_name,
    "type"::text,
    "definition"::text
FROM
    enum_defs
UNION ALL
SELECT
    schema_name,
    type_name,
    "type"::text,
    "definition"::text
FROM
    composite_defs
`

type GetDataTypesBySchemaAndTablesParams struct {
	Schema string
	Tables []string
}

type GetDataTypesBySchemaAndTablesRow struct {
	SchemaName string
	TypeName   string
	Type       string
	Definition string
}

func (q *Queries) GetDataTypesBySchemaAndTables(ctx context.Context, db DBTX, arg *GetDataTypesBySchemaAndTablesParams) ([]*GetDataTypesBySchemaAndTablesRow, error) {
	rows, err := db.Query(ctx, getDataTypesBySchemaAndTables, arg.Schema, arg.Tables)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetDataTypesBySchemaAndTablesRow
	for rows.Next() {
		var i GetDataTypesBySchemaAndTablesRow
		if err := rows.Scan(
			&i.SchemaName,
			&i.TypeName,
			&i.Type,
			&i.Definition,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getDatabaseSchema = `-- name: GetDatabaseSchema :many
WITH all_sequences AS (
    SELECT
        seq.relname AS sequence_name,
        nsp.nspname AS schema_name,
        seq.oid AS sequence_oid
    FROM
        pg_catalog.pg_class seq
    JOIN
        pg_catalog.pg_namespace nsp ON seq.relnamespace = nsp.oid
    WHERE
        seq.relkind = 'S'
),
linked_to_serial AS (
    SELECT
        seq.relname AS sequence_name,
        nsp.nspname AS schema_name,
        seq.oid AS sequence_oid,
        ad.adrelid,
        ad.adnum
    FROM
        pg_catalog.pg_class seq
    JOIN
        pg_catalog.pg_namespace nsp ON seq.relnamespace = nsp.oid
    JOIN
        pg_catalog.pg_depend dep ON dep.objid = seq.oid AND dep.classid = 'pg_catalog.pg_class'::regclass
    JOIN
        pg_catalog.pg_attrdef ad ON dep.refobjid = ad.adrelid AND dep.refobjsubid = ad.adnum
    WHERE
        pg_catalog.pg_get_expr(ad.adbin, ad.adrelid) LIKE 'nextval%'
),
column_defaults AS (
    SELECT
        n.nspname AS schema_name,
        c.relname AS table_name,
        a.attname AS column_name,
        pg_catalog.format_type(a.atttypid, a.atttypmod) AS data_type,
        COALESCE(pg_catalog.pg_get_expr(d.adbin, d.adrelid), '')::text AS column_default,
        CASE WHEN a.attnotnull THEN 'NO' ELSE 'YES' END AS is_nullable,
        CASE
            WHEN pg_catalog.format_type(a.atttypid, a.atttypmod) LIKE 'character varying%' THEN
                a.atttypmod - 4
            ELSE
                -1
        END AS character_maximum_length,
        CASE
            WHEN a.atttypid = pg_catalog.regtype 'numeric'::regtype THEN
                (a.atttypmod - 4) >> 16
            WHEN a.atttypid = pg_catalog.regtype 'smallint'::regtype THEN
                16
            WHEN a.atttypid = pg_catalog.regtype 'integer'::regtype THEN
                32
            WHEN a.atttypid = pg_catalog.regtype 'bigint'::regtype THEN
                64
            ELSE
                -1
        END AS numeric_precision,
        CASE
            WHEN a.atttypid = pg_catalog.regtype 'numeric'::regtype THEN
                CASE
                    WHEN (a.atttypmod) = -1 THEN -1
                    ELSE (a.atttypmod - 4) & 65535
                END
            WHEN a.atttypid = pg_catalog.regtype 'smallint'::regtype THEN
                0
            WHEN a.atttypid = pg_catalog.regtype 'integer'::regtype THEN
                0
            WHEN a.atttypid = pg_catalog.regtype 'bigint'::regtype THEN
                0
            ELSE
                -1
        END AS numeric_scale,
        a.attnum AS ordinal_position,
        a.attgenerated::text as generated_type,
        c.oid AS table_oid
    FROM
        pg_catalog.pg_attribute a
    INNER JOIN pg_catalog.pg_class c ON a.attrelid = c.oid
    INNER JOIN pg_catalog.pg_namespace n ON c.relnamespace = n.oid
    LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
    WHERE
        n.nspname NOT IN('pg_catalog', 'pg_toast', 'information_schema')
        AND a.attnum > 0
        AND NOT a.attisdropped
        AND c.relkind = 'r'
)
SELECT
    cd.schema_name, cd.table_name, cd.column_name, cd.data_type, cd.column_default, cd.is_nullable, cd.character_maximum_length, cd.numeric_precision, cd.numeric_scale, cd.ordinal_position, cd.generated_type, cd.table_oid,
    CASE
        WHEN ls.sequence_oid IS NOT NULL THEN 'SERIAL'
        WHEN cd.column_default LIKE 'nextval(%::regclass)' THEN 'USER-DEFINED SEQUENCE'
        ELSE ''
    END AS sequence_type
FROM
    column_defaults cd
LEFT JOIN linked_to_serial ls
    ON cd.table_oid = ls.adrelid
    AND cd.ordinal_position = ls.adnum
ORDER BY
    cd.ordinal_position
`

type GetDatabaseSchemaRow struct {
	SchemaName             string
	TableName              string
	ColumnName             string
	DataType               string
	ColumnDefault          string
	IsNullable             string
	CharacterMaximumLength int32
	NumericPrecision       int32
	NumericScale           int32
	OrdinalPosition        int16
	GeneratedType          string
	TableOid               pgtype.Uint32
	SequenceType           string
}

func (q *Queries) GetDatabaseSchema(ctx context.Context, db DBTX) ([]*GetDatabaseSchemaRow, error) {
	rows, err := db.Query(ctx, getDatabaseSchema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetDatabaseSchemaRow
	for rows.Next() {
		var i GetDatabaseSchemaRow
		if err := rows.Scan(
			&i.SchemaName,
			&i.TableName,
			&i.ColumnName,
			&i.DataType,
			&i.ColumnDefault,
			&i.IsNullable,
			&i.CharacterMaximumLength,
			&i.NumericPrecision,
			&i.NumericScale,
			&i.OrdinalPosition,
			&i.GeneratedType,
			&i.TableOid,
			&i.SequenceType,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getDatabaseTableSchemasBySchemasAndTables = `-- name: GetDatabaseTableSchemasBySchemasAndTables :many
WITH all_sequences AS (
    SELECT
        seq.relname AS sequence_name,
        nsp.nspname AS schema_name,
        seq.oid AS sequence_oid
    FROM
        pg_catalog.pg_class seq
    JOIN
        pg_catalog.pg_namespace nsp ON seq.relnamespace = nsp.oid
    WHERE
        seq.relkind = 'S'
),
linked_to_serial AS (
    SELECT
        seq.relname AS sequence_name,
        nsp.nspname AS schema_name,
        seq.oid AS sequence_oid,
        ad.adrelid,
        ad.adnum
    FROM
        pg_catalog.pg_class seq
    JOIN
        pg_catalog.pg_namespace nsp ON seq.relnamespace = nsp.oid
    JOIN
        pg_catalog.pg_depend dep ON dep.objid = seq.oid AND dep.classid = 'pg_catalog.pg_class'::regclass
    JOIN
        pg_catalog.pg_attrdef ad ON dep.refobjid = ad.adrelid AND dep.refobjsubid = ad.adnum
    WHERE
        pg_catalog.pg_get_expr(ad.adbin, ad.adrelid) LIKE 'nextval%'
),
column_defaults AS (
    SELECT
        n.nspname AS schema_name,
        c.relname AS table_name,
        a.attname AS column_name,
        pg_catalog.format_type(a.atttypid, a.atttypmod) AS data_type,
        COALESCE(pg_catalog.pg_get_expr(d.adbin, d.adrelid), '')::text AS column_default,
        CASE WHEN a.attnotnull THEN 'NO' ELSE 'YES' END AS is_nullable,
        CASE
            WHEN pg_catalog.format_type(a.atttypid, a.atttypmod) LIKE 'character varying%' THEN
                a.atttypmod - 4
            ELSE
                -1
        END AS character_maximum_length,
        CASE
            WHEN a.atttypid = pg_catalog.regtype 'numeric'::regtype THEN
                (a.atttypmod - 4) >> 16
            WHEN a.atttypid = pg_catalog.regtype 'smallint'::regtype THEN
                16
            WHEN a.atttypid = pg_catalog.regtype 'integer'::regtype THEN
                32
            WHEN a.atttypid = pg_catalog.regtype 'bigint'::regtype THEN
                64
            ELSE
                -1
        END AS numeric_precision,
        CASE
            WHEN a.atttypid = pg_catalog.regtype 'numeric'::regtype THEN
                CASE
                    WHEN (a.atttypmod) = -1 THEN -1
                    ELSE (a.atttypmod - 4) & 65535
                END
            WHEN a.atttypid = pg_catalog.regtype 'smallint'::regtype THEN
                0
            WHEN a.atttypid = pg_catalog.regtype 'integer'::regtype THEN
                0
            WHEN a.atttypid = pg_catalog.regtype 'bigint'::regtype THEN
                0
            ELSE
                -1
        END AS numeric_scale,
        a.attnum AS ordinal_position,
        a.attgenerated::text as generated_type,
        c.oid AS table_oid
    FROM
        pg_catalog.pg_attribute a
    INNER JOIN pg_catalog.pg_class c ON a.attrelid = c.oid
    INNER JOIN pg_catalog.pg_namespace n ON c.relnamespace = n.oid
    LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
    WHERE
        (n.nspname || '.' || c.relname) = ANY($1::TEXT[])
        AND a.attnum > 0
        AND NOT a.attisdropped
        AND c.relkind = 'r'
)
SELECT
    cd.schema_name, cd.table_name, cd.column_name, cd.data_type, cd.column_default, cd.is_nullable, cd.character_maximum_length, cd.numeric_precision, cd.numeric_scale, cd.ordinal_position, cd.generated_type, cd.table_oid,
    CASE
        WHEN ls.sequence_oid IS NOT NULL THEN 'SERIAL'
        WHEN cd.column_default LIKE 'nextval(%::regclass)' THEN 'USER-DEFINED SEQUENCE'
        ELSE ''
    END AS sequence_type
FROM
    column_defaults cd
LEFT JOIN linked_to_serial ls
    ON cd.table_oid = ls.adrelid
    AND cd.ordinal_position = ls.adnum
ORDER BY
    cd.ordinal_position
`

type GetDatabaseTableSchemasBySchemasAndTablesRow struct {
	SchemaName             string
	TableName              string
	ColumnName             string
	DataType               string
	ColumnDefault          string
	IsNullable             string
	CharacterMaximumLength int32
	NumericPrecision       int32
	NumericScale           int32
	OrdinalPosition        int16
	GeneratedType          string
	TableOid               pgtype.Uint32
	SequenceType           string
}

func (q *Queries) GetDatabaseTableSchemasBySchemasAndTables(ctx context.Context, db DBTX, schematables []string) ([]*GetDatabaseTableSchemasBySchemasAndTablesRow, error) {
	rows, err := db.Query(ctx, getDatabaseTableSchemasBySchemasAndTables, schematables)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetDatabaseTableSchemasBySchemasAndTablesRow
	for rows.Next() {
		var i GetDatabaseTableSchemasBySchemasAndTablesRow
		if err := rows.Scan(
			&i.SchemaName,
			&i.TableName,
			&i.ColumnName,
			&i.DataType,
			&i.ColumnDefault,
			&i.IsNullable,
			&i.CharacterMaximumLength,
			&i.NumericPrecision,
			&i.NumericScale,
			&i.OrdinalPosition,
			&i.GeneratedType,
			&i.TableOid,
			&i.SequenceType,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getIndicesBySchemasAndTables = `-- name: GetIndicesBySchemasAndTables :many
SELECT
    ns.nspname AS schema_name,
    t.relname AS table_name,
    i.relname AS index_name,
    pg_get_indexdef(ix.indexrelid) AS index_definition
FROM
    pg_catalog.pg_class t
    JOIN pg_catalog.pg_index ix ON t.oid = ix.indrelid
    JOIN pg_catalog.pg_class i ON i.oid = ix.indexrelid
    JOIN pg_catalog.pg_namespace ns ON t.relnamespace = ns.oid
LEFT JOIN pg_catalog.pg_constraint con ON con.conindid = ix.indexrelid
WHERE
    con.conindid IS NULL -- Excludes indexes created as part of constraints
    AND (ns.nspname || '.' || t.relname) = ANY($1::TEXT[])
GROUP BY
    ns.nspname, t.relname, i.relname, ix.indexrelid
ORDER BY
    schema_name,
    table_name,
    index_name
`

type GetIndicesBySchemasAndTablesRow struct {
	SchemaName      string
	TableName       string
	IndexName       string
	IndexDefinition string
}

func (q *Queries) GetIndicesBySchemasAndTables(ctx context.Context, db DBTX, schematables []string) ([]*GetIndicesBySchemasAndTablesRow, error) {
	rows, err := db.Query(ctx, getIndicesBySchemasAndTables, schematables)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetIndicesBySchemasAndTablesRow
	for rows.Next() {
		var i GetIndicesBySchemasAndTablesRow
		if err := rows.Scan(
			&i.SchemaName,
			&i.TableName,
			&i.IndexName,
			&i.IndexDefinition,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getPostgresRolePermissions = `-- name: GetPostgresRolePermissions :many
SELECT
    rtg.table_schema as table_schema,
    rtg.table_name as table_name,
    rtg.privilege_type as privilege_type
FROM
    information_schema.role_table_grants as rtg
WHERE
    table_schema NOT IN ('pg_catalog', 'information_schema')
AND grantee =  $1
ORDER BY
    table_schema,
    table_name
`

type GetPostgresRolePermissionsRow struct {
	TableSchema   string
	TableName     string
	PrivilegeType string
}

func (q *Queries) GetPostgresRolePermissions(ctx context.Context, db DBTX, role interface{}) ([]*GetPostgresRolePermissionsRow, error) {
	rows, err := db.Query(ctx, getPostgresRolePermissions, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetPostgresRolePermissionsRow
	for rows.Next() {
		var i GetPostgresRolePermissionsRow
		if err := rows.Scan(&i.TableSchema, &i.TableName, &i.PrivilegeType); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getTableConstraints = `-- name: GetTableConstraints :many
SELECT
    con.conname AS constraint_name,
    con.contype::TEXT AS constraint_type,
    con.connamespace::regnamespace::TEXT AS schema_name,
    cls.relname AS table_name,
    array_agg(att.attname)::TEXT[] AS constraint_columns,
    array_agg(att.attnotnull)::BOOL[] AS notnullable,
    CASE
        WHEN con.contype = 'f' THEN fn_cl.relnamespace::regnamespace::TEXT
        ELSE ''
    END AS foreign_schema_name,
    CASE
        WHEN con.contype = 'f' THEN fn_cl.relname
        ELSE ''
    END AS foreign_table_name,
    CASE
        WHEN con.contype = 'f' THEN fk_columns.foreign_column_names
        ELSE NULL::text[]
    END as foreign_column_names,
    pg_get_constraintdef(con.oid)::TEXT AS constraint_definition
FROM
    pg_catalog.pg_constraint con
JOIN
    pg_catalog.pg_attribute att ON
    att.attrelid = con.conrelid
    AND att.attnum = ANY(con.conkey)
JOIN
    pg_catalog.pg_class cls ON
    con.conrelid = cls.oid
JOIN
    pg_catalog.pg_namespace nsp ON
    cls.relnamespace = nsp.oid
LEFT JOIN
    pg_catalog.pg_class fn_cl ON
    fn_cl.oid = con.confrelid
LEFT JOIN LATERAL (
        SELECT
            array_agg(fk_att.attname) AS foreign_column_names
        FROM
            pg_catalog.pg_attribute fk_att
        WHERE
            fk_att.attrelid = con.confrelid
            AND fk_att.attnum = ANY(con.confkey)
    ) AS fk_columns ON
    TRUE
WHERE
    con.connamespace::regnamespace::TEXT = $1
    AND con.conrelid::regclass::TEXT = $2
GROUP BY
    con.oid,
    con.connamespace,
    con.conname,
    cls.relname,
    con.contype,
    fn_cl.relnamespace,
    fn_cl.relname,
    fk_columns.foreign_column_names
`

type GetTableConstraintsParams struct {
	Schema string
	Table  string
}

type GetTableConstraintsRow struct {
	ConstraintName       string
	ConstraintType       string
	SchemaName           string
	TableName            string
	ConstraintColumns    []string
	Notnullable          []bool
	ForeignSchemaName    string
	ForeignTableName     string
	ForeignColumnNames   []string
	ConstraintDefinition string
}

func (q *Queries) GetTableConstraints(ctx context.Context, db DBTX, arg *GetTableConstraintsParams) ([]*GetTableConstraintsRow, error) {
	rows, err := db.Query(ctx, getTableConstraints, arg.Schema, arg.Table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetTableConstraintsRow
	for rows.Next() {
		var i GetTableConstraintsRow
		if err := rows.Scan(
			&i.ConstraintName,
			&i.ConstraintType,
			&i.SchemaName,
			&i.TableName,
			&i.ConstraintColumns,
			&i.Notnullable,
			&i.ForeignSchemaName,
			&i.ForeignTableName,
			&i.ForeignColumnNames,
			&i.ConstraintDefinition,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getTableConstraintsBySchema = `-- name: GetTableConstraintsBySchema :many
SELECT
    con.conname AS constraint_name,
    con.contype::TEXT AS constraint_type,
    con.connamespace::regnamespace::TEXT AS schema_name,
    cls.relname AS table_name,
    array_agg(att.attname)::TEXT[] AS constraint_columns,
    array_agg(att.attnotnull)::BOOL[] AS notnullable,
    CASE
        WHEN con.contype = 'f' THEN fn_cl.relnamespace::regnamespace::TEXT
        ELSE ''
    END AS foreign_schema_name,
    CASE
        WHEN con.contype = 'f' THEN fn_cl.relname
        ELSE ''
    END AS foreign_table_name,
    CASE
        WHEN con.contype = 'f' THEN fk_columns.foreign_column_names
        ELSE NULL::text[]
    END as foreign_column_names,
    pg_get_constraintdef(con.oid)::TEXT AS constraint_definition
FROM
    pg_catalog.pg_constraint con
JOIN
    pg_catalog.pg_attribute att ON
    att.attrelid = con.conrelid
    AND att.attnum = ANY(con.conkey)
JOIN
    pg_catalog.pg_class cls ON
    con.conrelid = cls.oid
JOIN
    pg_catalog.pg_namespace nsp ON
    cls.relnamespace = nsp.oid
LEFT JOIN
    pg_catalog.pg_class fn_cl ON
    fn_cl.oid = con.confrelid
LEFT JOIN LATERAL (
        SELECT
            array_agg(fk_att.attname) AS foreign_column_names
        FROM
            pg_catalog.pg_attribute fk_att
        WHERE
            fk_att.attrelid = con.confrelid
            AND fk_att.attnum = ANY(con.confkey)
    ) AS fk_columns ON
    TRUE
WHERE
    con.connamespace::regnamespace::TEXT = ANY(
        $1::TEXT[]
    )
GROUP BY
    con.oid,
    con.connamespace,
    con.conname,
    cls.relname,
    con.contype,
    fn_cl.relnamespace,
    fn_cl.relname,
    fk_columns.foreign_column_names
`

type GetTableConstraintsBySchemaRow struct {
	ConstraintName       string
	ConstraintType       string
	SchemaName           string
	TableName            string
	ConstraintColumns    []string
	Notnullable          []bool
	ForeignSchemaName    string
	ForeignTableName     string
	ForeignColumnNames   []string
	ConstraintDefinition string
}

func (q *Queries) GetTableConstraintsBySchema(ctx context.Context, db DBTX, schema []string) ([]*GetTableConstraintsBySchemaRow, error) {
	rows, err := db.Query(ctx, getTableConstraintsBySchema, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetTableConstraintsBySchemaRow
	for rows.Next() {
		var i GetTableConstraintsBySchemaRow
		if err := rows.Scan(
			&i.ConstraintName,
			&i.ConstraintType,
			&i.SchemaName,
			&i.TableName,
			&i.ConstraintColumns,
			&i.Notnullable,
			&i.ForeignSchemaName,
			&i.ForeignTableName,
			&i.ForeignColumnNames,
			&i.ConstraintDefinition,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
