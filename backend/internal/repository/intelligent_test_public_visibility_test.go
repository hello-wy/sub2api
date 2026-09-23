package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestIntelligentPublicSubscriptionAccessMatchesAvailableGroups(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	_, err := db.Exec(`UPDATE test_settings SET user_visible=true;
UPDATE groups SET subscription_type='subscription' WHERE id=100;
DELETE FROM user_allowed_groups WHERE user_id=2;
UPDATE users SET restrict_public_groups=true WHERE id=2;
INSERT INTO user_subscriptions VALUES(2,100,'active',NOW()+INTERVAL '1 day',NULL);
INSERT INTO account_tests(id,account_id,test_type,status,result,input,config_snapshot,anti_degradation,requested_by,finished_at)
VALUES(901,10,'candy','completed','ANSWER: 12','private prompt','{}',true,1,NOW());`)
	require.NoError(t, err)
	f := service.IntelligentTestFilter{AccountID: 10, Page: 1, PageSize: 12}
	assertVisible := func(want bool) {
		t.Helper()
		items, total, listErr := repo.Capabilities(ctx, 2, f)
		require.NoError(t, listErr)
		history, historyErr := repo.PublicRecords(ctx, 2, f)
		require.NoError(t, historyErr)
		_, detailErr := repo.PublicGet(ctx, 2, 901)
		if want {
			require.EqualValues(t, 1, total)
			require.Len(t, items, 1)
			require.Len(t, history.Items, 1)
			require.NoError(t, detailErr)
		} else {
			require.Zero(t, total)
			require.Empty(t, items)
			require.Empty(t, history.Items)
			require.ErrorIs(t, detailErr, service.ErrIntelligentTestNotFound)
		}
	}
	assertVisible(true) // An active subscription is sufficient even for restricted users.
	_, err = db.Exec(`UPDATE user_subscriptions SET expires_at=NOW()-INTERVAL '1 second'; INSERT INTO user_allowed_groups VALUES(2,100)`)
	require.NoError(t, err)
	assertVisible(false) // An allowlist entry cannot revive an expired subscription.
	_, err = db.Exec(`UPDATE user_subscriptions SET expires_at=NOW()+INTERVAL '1 day'; UPDATE test_settings SET user_visible=false`)
	require.NoError(t, err)
	assertVisible(false)
	_, err = db.Exec(`UPDATE test_settings SET user_visible=true; UPDATE users SET status='disabled' WHERE id=2`)
	require.NoError(t, err)
	assertVisible(false)
	_, err = db.Exec(`UPDATE users SET status='active' WHERE id=2; UPDATE groups SET status='disabled' WHERE id=100`)
	require.NoError(t, err)
	assertVisible(false)
	_, err = db.Exec(`UPDATE groups SET status='active',subscription_type='standard' WHERE id=100; DELETE FROM user_allowed_groups WHERE user_id=2`)
	require.NoError(t, err)
	assertVisible(false) // Subscription rows must not grant access to standard exclusive groups.
	_, err = db.Exec(`INSERT INTO user_allowed_groups VALUES(2,100)`)
	require.NoError(t, err)
	assertVisible(true)
	_, err = db.Exec(`UPDATE groups SET is_exclusive=false WHERE id=100; DELETE FROM user_allowed_groups WHERE user_id=2`)
	require.NoError(t, err)
	assertVisible(false) // Restricted public groups still require explicit permission.
	_, err = db.Exec(`UPDATE users SET restrict_public_groups=false WHERE id=2`)
	require.NoError(t, err)
	assertVisible(true)
}

func TestIntelligentPublicPelicanKeepsLastPictureAndSanitizesHTML(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	_, err := db.Exec(`UPDATE test_settings SET user_visible=true WHERE test_type='pelican'`)
	require.NoError(t, err)
	legacyHTML := `<html><body>private explanation<script>fetch('/private')</script><svg viewBox="0 0 100 100"><style>.bird{fill:orange}</style><circle class="bird" cx="50" cy="50" r="20"/><script>alert(1)</script></svg></body></html>`
	_, err = db.Exec(`INSERT INTO account_tests(id,account_id,test_type,status,score,result,input,config_snapshot,evaluation,anti_degradation,requested_by,finished_at)
VALUES(911,10,'pelican','success',100,$1,'private prompt','{}','{"evaluator_version":3,"answer_verdict":"correct"}',true,1,NOW()),
(912,10,'pelican','network_error',NULL,'','private prompt','{}','{}',true,1,NOW()),
(913,10,'pelican','cancelled',NULL,$1,'private prompt','{}','{}',true,1,NOW()),
(914,10,'pelican','running',NULL,'','private prompt','{}','{}',true,1,NULL),
(915,10,'pelican','completed',NULL,'<svg><text>not a drawing</text></svg>','private prompt','{}','{"image_state":"unavailable"}',true,1,NOW())`, legacyHTML)
	require.NoError(t, err)
	f := service.IntelligentTestFilter{Page: 1, PageSize: 12}
	items, total, err := repo.Capabilities(ctx, 2, f)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Len(t, items[0].Tests, 1)
	require.EqualValues(t, 911, items[0].Tests[0].ID, "new failed/cancelled/pending runs must keep the last picture visible")
	image := items[0].Tests[0]
	require.Empty(t, image.Result)
	require.Nil(t, image.Score)
	require.Nil(t, image.Evaluation)
	require.Contains(t, image.ResultImage, `fill="orange"`)
	require.NotContains(t, image.ResultImage, "script")
	require.NotContains(t, image.ResultImage, "private")
	f.AccountID = 10
	history, err := repo.PublicRecords(ctx, 2, f)
	require.NoError(t, err)
	require.EqualValues(t, 1, history.Total)
	require.Len(t, history.Items, 1)
	public, err := repo.PublicGet(ctx, 2, 911)
	require.NoError(t, err)
	require.Equal(t, image.ResultImage, public.ResultImage)
	for _, id := range []int64{912, 913, 914, 915} {
		_, err := repo.PublicGet(ctx, 2, id)
		require.ErrorIs(t, err, service.ErrIntelligentTestNotFound)
	}
	_, err = repo.PublicGet(ctx, 3, 911)
	require.ErrorIs(t, err, service.ErrIntelligentTestNotFound, "image visibility still requires current group permission")
	_, err = db.Exec(`UPDATE test_settings SET user_visible=false WHERE test_type='pelican'`)
	require.NoError(t, err)
	_, err = repo.PublicGet(ctx, 2, 911)
	require.ErrorIs(t, err, service.ErrIntelligentTestNotFound)
}
