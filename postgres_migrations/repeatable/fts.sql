DROP TRIGGER IF EXISTS note_before_insert_update_trigger ON note;

CREATE TRIGGER note_before_insert_update_trigger BEFORE INSERT OR UPDATE ON note
FOR EACH ROW EXECUTE PROCEDURE tsvector_update_trigger(fts, 'pg_catalog.english', body);
