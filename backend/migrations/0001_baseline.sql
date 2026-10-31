-- 0001_baseline: shared helpers used by later table migrations.

-- Keeps updated_at accurate without relying on application code.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
