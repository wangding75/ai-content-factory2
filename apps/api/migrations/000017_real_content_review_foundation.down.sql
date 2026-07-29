DO $$
BEGIN
    RAISE EXCEPTION 'Migration 000017 is forward-only and must not be downgraded';
END
$$;
