-- name: GetDatabaseSchema :many
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
    cd.*,
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
    cd.ordinal_position;

-- name: GetDatabaseTableSchemasBySchemasAndTables :many
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
        (n.nspname || '.' || c.relname) = ANY(sqlc.arg('schematables')::TEXT[])
        AND a.attnum > 0
        AND NOT a.attisdropped
        AND c.relkind = 'r'
)
SELECT
    cd.*,
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
    cd.ordinal_position;

-- name: GetTableConstraints :many
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
    con.connamespace::regnamespace::TEXT = sqlc.arg('schema')
    AND con.conrelid::regclass::TEXT = sqlc.arg('table')
GROUP BY
    con.oid,
    con.connamespace,
    con.conname,
    cls.relname,
    con.contype,
    fn_cl.relnamespace,
    fn_cl.relname,
    fk_columns.foreign_column_names;

-- name: GetTableConstraintsBySchema :many
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
        sqlc.arg('schema')::TEXT[]
    )
GROUP BY
    con.oid,
    con.connamespace,
    con.conname,
    cls.relname,
    con.contype,
    fn_cl.relnamespace,
    fn_cl.relname,
    fk_columns.foreign_column_names;

-- name: GetPostgresRolePermissions :many
SELECT
    rtg.table_schema as table_schema,
    rtg.table_name as table_name,
    rtg.privilege_type as privilege_type
FROM
    information_schema.role_table_grants as rtg
WHERE
    table_schema NOT IN ('pg_catalog', 'information_schema')
AND grantee =  sqlc.arg('role')
ORDER BY
    table_schema,
    table_name;

-- name: GetIndicesBySchemasAndTables :many
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
    AND (ns.nspname || '.' || t.relname) = ANY(sqlc.arg('schematables')::TEXT[])
GROUP BY
    ns.nspname, t.relname, i.relname, ix.indexrelid
ORDER BY
    schema_name,
    table_name,
    index_name;

-- name: GetDataTypesBySchemaAndTables :many
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
        n.nspname = sqlc.arg('schema')
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
        n.nspname = sqlc.arg('schema')
        AND c.relname = ANY(sqlc.arg('tables')::TEXT[])
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
    composite_defs;

-- name: GetCustomFunctionsBySchemaAndTables :many
WITH relevant_schemas_tables AS (
    SELECT c.oid, n.nspname AS schema_name, c.relname AS table_name
    FROM pg_catalog.pg_class c
    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = sqlc.arg('schema')
    AND c.relname = ANY(sqlc.arg('tables')::TEXT[])
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
    function_name;


-- name: GetCustomTriggersBySchemaAndTables :many
SELECT
    n.nspname AS schema_name,
    c.relname AS table_name,
    t.tgname AS trigger_name,
    pg_catalog.pg_get_triggerdef(t.oid, true) AS definition
FROM pg_catalog.pg_trigger t
JOIN pg_catalog.pg_class c ON c.oid = t.tgrelid
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE  (n.nspname || '.' || c.relname) = ANY(sqlc.arg('schematables')::TEXT[])
AND NOT t.tgisinternal
ORDER BY
    schema_name,
    table_name,
    trigger_name;

-- name: GetCustomSequencesBySchemaAndTables :many
WITH relevant_schemas_tables AS (
    SELECT c.oid, n.nspname AS schema_name, c.relname AS table_name
    FROM pg_catalog.pg_class c
    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = sqlc.arg('schema')
    AND c.relname = ANY(sqlc.arg('tables')::TEXT[])
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
    cs.sequence_name;
