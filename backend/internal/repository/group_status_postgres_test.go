//go:build unit

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestGroupStatusPostgresCompletionAggregationScopesFreeFailuresAndRetries(t *testing.T) {
	dsn := os.Getenv("GROUP_STATUS_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("GROUP_STATUS_POSTGRES_TEST_DSN is not set")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer admin.Close()
	schema := "status_" + uuid.NewString()[:8]
	_, err = admin.Exec("CREATE SCHEMA " + pq.QuoteIdentifier(schema))
	require.NoError(t, err)
	defer admin.Exec("DROP SCHEMA " + pq.QuoteIdentifier(schema) + " CASCADE")
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	q := parsed.Query()
	q.Set("search_path", schema)
	parsed.RawQuery = q.Encode()
	db, err := sql.Open("postgres", parsed.String())
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE groups(id bigint primary key,name text,platform text);CREATE TABLE users(id bigint primary key);CREATE TABLE accounts(id bigint primary key,platform text);
 CREATE TABLE usage_logs(id bigserial primary key,request_id text,user_id bigint,group_id bigint,account_id bigint,created_at timestamptz,requested_model text,model text,request_type integer default 2,input_tokens bigint default 0,output_tokens bigint default 0,cache_read_tokens bigint default 0,cache_creation_tokens bigint default 0,actual_cost numeric default 0,image_count integer default 0,video_count integer default 0,stream boolean default true,first_token_ms bigint,duration_ms bigint);
 CREATE TABLE ops_error_logs(id bigserial primary key,request_id text,client_request_id text,group_id bigint,user_id bigint,account_id bigint,platform text,model text,requested_model text,error_type text,error_owner text,status_code integer,upstream_status_code integer,error_source text,error_message text,upstream_error_message text,upstream_error_detail text,error_body text,upstream_errors jsonb,created_at timestamptz,is_count_tokens boolean default false);
 INSERT INTO groups VALUES(1,'group-one','openai'),(2,'group-two','openai');INSERT INTO users VALUES(1),(2);INSERT INTO accounts VALUES(1,'openai');`)
	require.NoError(t, err)
	for _, name := range []string{"194_channel_monitor_v2.sql", "199_channel_monitor_v2_fixed_rollups.sql", "252_group_service_status.sql", "254_group_status_request_identity.sql"} {
		data, e := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		require.NoError(t, e)
		_, e = db.Exec(string(data))
		require.NoError(t, e, name)
	}
	at := time.Now().UTC().Truncate(time.Minute).Add(-3 * time.Minute)
	ctx := context.Background()
	// Two users deliberately reuse the same client request id. A retry is a
	// duplicate usage row for user 1, while user 2's partial bill ends in failure.
	fixture := `INSERT INTO usage_logs(request_id,user_id,group_id,account_id,created_at,model,input_tokens,output_tokens,first_token_ms,duration_ms) VALUES
 ('client:shared',1,1,1,$1,'gpt-5',10,20,1000,2000),('client:shared',1,1,1,$1,'gpt-5',10,20,1000,2000),
 ('client:shared',2,1,1,$1,'gpt-5',10,20,1000,2000),('client:legacy',1,1,1,$1,'gpt-5',10,20,1000,2000),('client:legacy',2,1,1,$1,'gpt-5',10,20,1000,2000);
 INSERT INTO channel_monitor_request_outcomes(request_id,user_id,group_id,platform,model,success,error_category,completed_at) VALUES
 ('client:shared',1,1,'openai','gpt-5',true,'',$1),('client:shared',2,1,'openai','gpt-5',false,'transport_or_stream',$1),('client:free-zero',1,1,'openai','gpt-5',true,'',$1),('client:shared',1,2,'openai','gpt-5',false,'rate_or_capacity',$1);
 INSERT INTO ops_error_logs(request_id,client_request_id,user_id,group_id,account_id,platform,model,status_code,error_type,created_at) VALUES('server-2','shared',2,1,1,'openai','gpt-5',502,'stream_read_error',$1);
 INSERT INTO usage_logs(request_id,user_id,group_id,account_id,created_at,model,actual_cost,stream) VALUES
 ('grok_audio:a',1,1,1,$1,'grok-tts',0.1,false),('grok_audio:b',1,1,1,$1,'grok-tts',0.1,false),
 ('web_search:a',1,1,1,$1,'grok-web-search',0.1,false),('web_search:b',1,1,1,$1,'grok-web-search',0.1,false),
 ('grok_audio:failed',1,1,1,$1,'grok-stt',0.1,false);
 INSERT INTO channel_monitor_request_outcomes(request_id,gateway_request_id,user_id,group_id,platform,model,success,error_category,completed_at) VALUES
 ('grok_audio:a','client:reused',1,1,'openai','grok-tts',true,'',$1),('grok_audio:b','client:reused',1,1,'openai','grok-tts',true,'',$1),
 ('web_search:a','client:reused',1,1,'openai','grok-web-search',true,'',$1),('web_search:b','client:reused',1,1,'openai','grok-web-search',true,'',$1),
 ('grok_audio:failed','client:audio-failed',1,1,'openai','grok-stt',false,'transport_or_stream',$1);
 INSERT INTO ops_error_logs(request_id,client_request_id,user_id,group_id,account_id,platform,model,status_code,error_type,created_at) VALUES('server-audio','audio-failed',1,1,1,'openai','grok-stt',502,'stream_read_error',$1);`
	for _, query := range strings.Split(fixture, ";") {
		if strings.TrimSpace(query) == "" {
			continue
		}
		_, err = db.Exec(query, at)
		require.NoError(t, err)
	}
	start, end := at.Add(-time.Minute), at.Add(time.Minute)
	for _, query := range []string{fmt.Sprintf(channelMonitorV2UsageMetricsSQL, channelMonitorV2PlatformSQL, channelMonitorV2ModelSQL), fmt.Sprintf(channelMonitorV2UserMetricsSQL, channelMonitorV2PlatformSQL, channelMonitorV2ModelSQL), fmt.Sprintf(channelMonitorV2HistogramSQL, channelMonitorV2PlatformSQL, channelMonitorV2ModelSQL, channelMonitorV2HistogramBoundSQL("latency.value_ms")), channelMonitorV2ErrorAggregationSQL, channelMonitorV2OutcomeAggregationSQL} {
		_, err = db.ExecContext(ctx, query, start, end)
		require.NoError(t, err)
	}
	var successes, failures, samples int64
	var speed float64
	err = db.QueryRow(`SELECT success_requests,error_requests,generation_tps_sum,generation_tps_count FROM channel_monitor_v2_metrics_1m WHERE group_id=1 AND model='gpt-5'`).Scan(&successes, &failures, &speed, &samples)
	require.NoError(t, err)
	require.Equal(t, int64(4), successes, "free-zero plus shared user 1 and both legacy users count once each")
	require.Equal(t, int64(1), failures, "partial billed stream error counted once, not duplicated by ops")
	require.Equal(t, int64(3), samples)
	require.Equal(t, 60.0, speed)
	err = db.QueryRow(`SELECT success_requests,error_requests FROM channel_monitor_v2_metrics_1m WHERE group_id=2`).Scan(&successes, &failures)
	require.NoError(t, err)
	require.Zero(t, successes)
	require.Equal(t, int64(1), failures)
	_, err = db.Exec(channelMonitorV2MetricsRollupSQL, "3600 seconds", 3600, start, end)
	require.NoError(t, err)
	var rollupSpeed float64
	err = db.QueryRow(`SELECT generation_tps_sum FROM channel_monitor_v2_metrics_rollup WHERE group_id=1 AND model='gpt-5' LIMIT 1`).Scan(&rollupSpeed)
	require.NoError(t, err)
	require.Equal(t, 60.0, rollupSpeed)
	for _, model := range []string{"grok-tts", "grok-web-search"} {
		require.NoError(t, db.QueryRow(`SELECT success_requests,error_requests FROM channel_monitor_v2_metrics_1m WHERE group_id=1 AND model=$1`, model).Scan(&successes, &failures))
		require.Equal(t, int64(2), successes, "each forced money event counts once despite reused client correlation")
		require.Zero(t, failures)
	}
	require.NoError(t, db.QueryRow(`SELECT success_requests,error_requests FROM channel_monitor_v2_metrics_1m WHERE group_id=1 AND model='grok-stt'`).Scan(&successes, &failures))
	require.Zero(t, successes)
	require.Equal(t, int64(1), failures, "gateway alias joins the ops error without duplicating the authoritative completion")
}
