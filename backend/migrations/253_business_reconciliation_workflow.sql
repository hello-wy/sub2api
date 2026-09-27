-- Read models only: original financial events and saved prices stay immutable.
CREATE INDEX IF NOT EXISTS business_annotations_source ON business_events ((payload->>'source_event_id'), id) WHERE event_type='annotation';
CREATE INDEX IF NOT EXISTS business_events_reversal ON business_events (reverses_id) WHERE reverses_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS business_usage_account_time ON business_events ((payload->>'account_id'), occurred_at, id) WHERE event_type='usage';
CREATE INDEX IF NOT EXISTS business_entries_kind_time ON business_ledger_entries (kind, occurred_at);

CREATE OR REPLACE VIEW business_applied_annotations AS
SELECT source_id, jsonb_object_agg(field, value) AS fields
FROM (
  SELECT DISTINCT ON (a.payload->>'source_event_id', f.key)
    a.payload->>'source_event_id' AS source_id, f.key AS field, f.value
  FROM business_events a
  JOIN business_projection_processed done ON done.event_id=a.id
  CROSS JOIN LATERAL jsonb_each(a.payload->'fields') f
  WHERE a.event_type='annotation'
  ORDER BY a.payload->>'source_event_id', f.key, a.id DESC
) latest GROUP BY source_id;

-- Match the report's invoice coverage, including a correction after verification.
CREATE OR REPLACE VIEW business_effective_cost_entries AS
WITH bills AS (
  SELECT bill->'payload' AS payload FROM business_projection_state s,
  LATERAL jsonb_array_elements(COALESCE(NULLIF(s.state->'bills','null'::jsonb),'[]'::jsonb)) bill WHERE s.id=1
), amounts AS (
  SELECT a.key AS id, a.value FROM bills, LATERAL jsonb_each(payload->'covered_entry_amounts') a
), covered AS (
  SELECT DISTINCT id FROM bills, LATERAL jsonb_array_elements_text(payload->'covered_entry_ids') id
), events AS (
  SELECT DISTINCT event_id::bigint AS id FROM bills, LATERAL jsonb_array_elements_text(payload->'covered_event_ids') event_id
), checked AS (
  SELECT l.*, (c.id IS NOT NULL) AS covered,
    ((a.id IS NOT NULL AND (CASE WHEN a.value='null'::jsonb THEN NULL ELSE (a.value #>> '{}')::numeric END IS DISTINCT FROM l.amount_cny))
     OR (a.id IS NULL AND e.id IS NOT NULL)) AS invoice_stale
  FROM business_ledger_entries l
  LEFT JOIN amounts a ON a.id=l.id LEFT JOIN covered c ON c.id=l.id LEFT JOIN events e ON e.id=l.event_id
  WHERE l.kind IN ('usage_cost','cost_gap')
)
SELECT checked.*, (covered AND NOT invoice_stale) AS invoice_verified FROM checked;

-- Source tasks point to the original receipt/funding/term, never a consuming request.
CREATE OR REPLACE VIEW business_unresolved_sources AS
SELECT e.id, e.source_key, e.event_type, e.transaction_id, e.user_id, e.occurred_at, e.recorded_at,
       e.payload || COALESCE(a.fields,'{}'::jsonb) AS payload, e.actor_id, e.reverses_id
FROM business_events e
JOIN business_projection_processed done ON done.event_id=e.id
LEFT JOIN business_applied_annotations a ON a.source_id=e.id::text
WHERE (
  (e.event_type='opening_unknown' AND COALESCE((a.fields->>'unknown_credits')::numeric,(e.payload->>'credits')::numeric,0)>0)
  OR (e.event_type IN ('wallet','payment_orders') AND EXISTS (
    SELECT 1 FROM business_ledger_entries l WHERE l.event_id=e.id AND l.kind IN ('funding_unknown','wallet_adjustment','cash_gap','revenue_gap')
  ))
  OR (e.event_type='user_subscriptions' AND EXISTS (
    SELECT 1 FROM business_projection_state p, LATERAL jsonb_each(COALESCE(p.state->'terms','{}'::jsonb)) t
    WHERE t.value->>'event_id'=e.id::text AND t.value->>'quality'='unknown'
  ))
) AND NOT EXISTS (SELECT 1 FROM business_events r JOIN business_projection_processed done ON done.event_id=r.id WHERE r.reverses_id=e.id);
