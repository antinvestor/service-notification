-- Templates are registered by name at consumer deploy time (TemplateSave upsert).
-- Collapse legacy duplicates and enforce uniqueness so repeated registrations are idempotent.
-- Frame executes this file as a single statement, hence the DO block.
DO $$
BEGIN
    -- (a) Duplicate templates sharing (tenant_id, partition_id, name): keep the oldest row,
    --     repoint dependants to it and drop the rest.
    CREATE TEMP TABLE template_dupes ON COMMIT DROP AS
    SELECT t.id AS dup_id, k.keep_id
    FROM templates t
    JOIN (
        SELECT tenant_id, partition_id, name,
               (array_agg(id ORDER BY created_at ASC, id ASC))[1] AS keep_id
        FROM templates
        WHERE deleted_at IS NULL
        GROUP BY tenant_id, partition_id, name
        HAVING count(*) > 1
    ) k ON k.tenant_id IS NOT DISTINCT FROM t.tenant_id
       AND k.partition_id IS NOT DISTINCT FROM t.partition_id
       AND k.name = t.name
    WHERE t.deleted_at IS NULL
      AND t.id <> k.keep_id;

    UPDATE template_data td
    SET template_id = d.keep_id
    FROM template_dupes d
    WHERE td.template_id = d.dup_id;

    UPDATE notifications n
    SET template_id = d.keep_id
    FROM template_dupes d
    WHERE n.template_id = d.dup_id;

    DELETE FROM templates t
    USING template_dupes d
    WHERE t.id = d.dup_id;

    -- (b) Duplicate template_data sharing (template_id, language_id, type): keep the most
    --     recently modified body (the latest save is the intended content).
    DELETE FROM template_data td
    USING (
        SELECT id,
               row_number() OVER (
                   PARTITION BY template_id, language_id, type
                   ORDER BY modified_at DESC, created_at DESC, id DESC
               ) AS rn
        FROM template_data
        WHERE deleted_at IS NULL
    ) ranked
    WHERE td.id = ranked.id
      AND ranked.rn > 1;

    -- (c) Enforce uniqueness going forward. The gorm tag `unique_index` on TemplateData is a
    --     gorm v1 tag and is ignored by gorm v2, so this index never existed.
    CREATE UNIQUE INDEX IF NOT EXISTS uq_templates_name_by_tenancy
        ON templates (tenant_id, partition_id, name)
        WHERE deleted_at IS NULL;

    CREATE UNIQUE INDEX IF NOT EXISTS uq_template_by_type
        ON template_data (template_id, language_id, type)
        WHERE deleted_at IS NULL;
END $$;
