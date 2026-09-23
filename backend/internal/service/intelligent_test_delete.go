package service

import "context"

// Kept separate from the worker contract: deleting history never cancels work.
type IntelligentTestDeleteRepository interface {
	DeleteRecords(context.Context, int64, []int64) (int64, error)
}

func (s *IntelligentTestService) DeleteRecords(ctx context.Context, actor int64, ids []int64) (int64, error) {
	if err := s.authorize(ctx, actor); err != nil {
		return 0, err
	}
	if len(ids) == 0 || len(ids) > 100 {
		return 0, intelligentTestBad("请选择 1–100 条测试记录")
	}
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id < 1 || seen[id] {
			return 0, intelligentTestBad("记录编号必须为正整数且不能重复")
		}
		seen[id] = true
	}
	repo, ok := s.repo.(IntelligentTestDeleteRepository)
	if !ok {
		return 0, intelligentTestBad("当前存储不支持删除测试记录")
	}
	return repo.DeleteRecords(ctx, actor, ids)
}
