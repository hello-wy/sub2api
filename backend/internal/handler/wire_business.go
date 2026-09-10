package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// AdminHandlersDependencies keeps local modules in the generated application graph.
type AdminHandlersDependencies struct {
	DashboardHandler              *admin.DashboardHandler
	UserHandler                   *admin.UserHandler
	GroupHandler                  *admin.GroupHandler
	AccountHandler                *admin.AccountHandler
	AnnouncementHandler           *admin.AnnouncementHandler
	DataManagementHandler         *admin.DataManagementHandler
	BackupHandler                 *admin.BackupHandler
	OAuthHandler                  *admin.OAuthHandler
	OpenAIOAuthHandler            *admin.OpenAIOAuthHandler
	GeminiOAuthHandler            *admin.GeminiOAuthHandler
	AntigravityOAuthHandler       *admin.AntigravityOAuthHandler
	GrokOAuthHandler              *admin.GrokOAuthHandler
	CNProviderHandler             *admin.CNProviderHandler
	ProxyHandler                  *admin.ProxyHandler
	RedeemHandler                 *admin.RedeemHandler
	PromoHandler                  *admin.PromoHandler
	SettingHandler                *admin.SettingHandler
	OpsHandler                    *admin.OpsHandler
	SystemHandler                 *admin.SystemHandler
	SubscriptionHandler           *admin.SubscriptionHandler
	UsageHandler                  *admin.UsageHandler
	UserAttributeHandler          *admin.UserAttributeHandler
	ErrorPassthroughHandler       *admin.ErrorPassthroughHandler
	TLSFingerprintProfileHandler  *admin.TLSFingerprintProfileHandler
	PluginHandler                 *admin.PluginHandler
	APIKeyHandler                 *admin.AdminAPIKeyHandler
	ScheduledTestHandler          *admin.ScheduledTestHandler
	ChannelHandler                *admin.ChannelHandler
	ChannelMonitorHandler         *admin.ChannelMonitorHandler
	ChannelMonitorTemplateHandler *admin.ChannelMonitorRequestTemplateHandler
	ContentModerationHandler      *admin.ContentModerationHandler
	PromptAuditHandler            *securityaudit.PromptAdminHandler
	PaymentHandler                *admin.PaymentHandler
	AffiliateHandler              *admin.AffiliateHandler
	WelfareHandler                *admin.WelfareHandler
	ComplianceHandler             *admin.ComplianceHandler
	AuditLogHandler               *admin.AuditLogHandler
	UpstreamBillingProbe          *service.UpstreamBillingProbeService
	OllamaCloudUsage              *service.OllamaCloudUsageService
}

func ProvideAdminHandlers(deps AdminHandlersDependencies) *AdminHandlers {
	deps.AccountHandler.SetUpstreamBillingProbeService(deps.UpstreamBillingProbe)
	deps.AccountHandler.SetOllamaCloudUsageService(deps.OllamaCloudUsage)
	return &AdminHandlers{
		Dashboard:              deps.DashboardHandler,
		User:                   deps.UserHandler,
		Group:                  deps.GroupHandler,
		Account:                deps.AccountHandler,
		Announcement:           deps.AnnouncementHandler,
		DataManagement:         deps.DataManagementHandler,
		Backup:                 deps.BackupHandler,
		OAuth:                  deps.OAuthHandler,
		OpenAIOAuth:            deps.OpenAIOAuthHandler,
		GeminiOAuth:            deps.GeminiOAuthHandler,
		AntigravityOAuth:       deps.AntigravityOAuthHandler,
		GrokOAuth:              deps.GrokOAuthHandler,
		CNProvider:             deps.CNProviderHandler,
		Proxy:                  deps.ProxyHandler,
		Redeem:                 deps.RedeemHandler,
		Promo:                  deps.PromoHandler,
		Setting:                deps.SettingHandler,
		Ops:                    deps.OpsHandler,
		System:                 deps.SystemHandler,
		Subscription:           deps.SubscriptionHandler,
		Usage:                  deps.UsageHandler,
		UserAttribute:          deps.UserAttributeHandler,
		ErrorPassthrough:       deps.ErrorPassthroughHandler,
		TLSFingerprintProfile:  deps.TLSFingerprintProfileHandler,
		Plugin:                 deps.PluginHandler,
		APIKey:                 deps.APIKeyHandler,
		ScheduledTest:          deps.ScheduledTestHandler,
		Channel:                deps.ChannelHandler,
		ChannelMonitor:         deps.ChannelMonitorHandler,
		ChannelMonitorTemplate: deps.ChannelMonitorTemplateHandler,
		ContentModeration:      deps.ContentModerationHandler,
		PromptAudit:            deps.PromptAuditHandler,
		Payment:                deps.PaymentHandler,
		Affiliate:              deps.AffiliateHandler,
		Welfare:                deps.WelfareHandler,
		Compliance:             deps.ComplianceHandler,
		AuditLog:               deps.AuditLogHandler,
	}
}

type AdminUserHandlerDependencies struct {
	AdminService          service.AdminService
	ConcurrencyService    *service.ConcurrencyService
	UserPlatformQuotaRepo service.UserPlatformQuotaRepository
	BillingCache          service.BillingCache
	TotpService           *service.TotpService
	UserService           *service.UserService
	SettingService        *service.SettingService
	LotteryService        *service.LotteryService
	QQBindingService      *service.QQBindingService
}

func ProvideAdminUserHandler(deps AdminUserHandlerDependencies) *admin.UserHandler {
	h := admin.NewUserHandler(deps.AdminService, deps.ConcurrencyService, deps.UserPlatformQuotaRepo, deps.BillingCache, deps.TotpService, deps.UserService, deps.SettingService)
	h.SetLotteryService(deps.LotteryService)
	h.SetQQBindingService(deps.QQBindingService)
	return h
}

type DashboardHandlerDependencies struct {
	DashboardService   *service.DashboardService
	AggregationService *service.DashboardAggregationService
	BusinessService    *service.BusinessAnalyticsService
}

func ProvideDashboardHandler(deps DashboardHandlerDependencies) *admin.DashboardHandler {
	handler := admin.NewDashboardHandler(deps.DashboardService, deps.AggregationService)
	handler.SetBusinessAnalyticsService(deps.BusinessService)
	return handler
}

type UserHandlerDependencies struct {
	UserService           *service.UserService
	AuthService           *service.AuthService
	EmailService          *service.EmailService
	EmailCache            service.EmailCache
	AffiliateService      *service.AffiliateService
	UserPlatformQuotaRepo service.UserPlatformQuotaRepository
	UserAttributeService  *service.UserAttributeService
	LotteryService        *service.LotteryService
}

func ProvideUserHandler(deps UserHandlerDependencies) *UserHandler {
	h := NewUserHandler(deps.UserService, deps.AuthService, deps.EmailService, deps.EmailCache, deps.AffiliateService, deps.UserPlatformQuotaRepo, deps.UserAttributeService)
	h.SetLotteryService(deps.LotteryService)
	return h
}

type AdminSettingHandlerDependencies struct {
	SettingService           *service.SettingService
	EmailService             *service.EmailService
	TurnstileService         *service.TurnstileService
	AliyunCaptchaService     *service.AliyunCaptchaService
	OpsService               *service.OpsService
	PaymentConfigService     *service.PaymentConfigService
	PaymentService           *service.PaymentService
	UserAttributeService     *service.UserAttributeService
	NotificationEmailService *service.NotificationEmailService
	TotpService              *service.TotpService
	UserService              *service.UserService
	LotteryService           *service.LotteryService
}

func ProvideAdminSettingHandler(deps AdminSettingHandlerDependencies) *admin.SettingHandler {
	h := admin.NewSettingHandler(deps.SettingService, deps.EmailService, deps.TurnstileService, deps.OpsService, deps.PaymentConfigService, deps.PaymentService, deps.UserAttributeService)
	h.SetNotificationEmailService(deps.NotificationEmailService)
	h.SetAliyunCaptchaService(deps.AliyunCaptchaService)
	h.SetStepUpDeps(deps.TotpService, deps.UserService)
	h.SetLotteryService(deps.LotteryService)
	return h
}
