package cronjob

import (
	"context"

	"github.com/deposist/s-ui-x-extended/logger"
	"github.com/deposist/s-ui-x-extended/service"
)

type subscriptionRefreshJob struct{ ctx context.Context }

func (j *subscriptionRefreshJob) Run() {
	if err := (&service.ConfigService{}).RefreshSubscriptions(j.ctx, false); err != nil && j.ctx.Err() == nil {
		logger.Warning("subscription refresh failed: ", err)
	}
}
