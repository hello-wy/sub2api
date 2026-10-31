-- PostgreSQL's numeric parser preserves decimals and large integers. Invalid,
-- empty, out-of-range and non-finite values sort as NULL instead of failing a list.
CREATE OR REPLACE FUNCTION user_attribute_numeric_value(raw_value TEXT)
RETURNS NUMERIC
LANGUAGE plpgsql IMMUTABLE STRICT AS $$
DECLARE
    parsed NUMERIC;
BEGIN
    parsed := raw_value::NUMERIC;
    IF parsed IN ('NaN'::NUMERIC, 'Infinity'::NUMERIC, '-Infinity'::NUMERIC) THEN
        RETURN NULL;
    END IF;
    RETURN parsed;
EXCEPTION WHEN invalid_text_representation OR numeric_value_out_of_range THEN
    RETURN NULL;
END;
$$;
