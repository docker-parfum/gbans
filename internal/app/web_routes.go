package app

import (
	"github.com/Depado/ginprom"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/leighmacdonald/gbans/internal/config"
	"github.com/leighmacdonald/gbans/internal/model"
	"github.com/leighmacdonald/gbans/internal/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

func prometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(ctx *gin.Context) {
		h.ServeHTTP(ctx.Writer, ctx.Request)
	}
}

var registered = false

func (web *web) setupRouter(database store.Store, engine *gin.Engine, logFileC chan *LogFilePayload) {
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = config.HTTP.CorsOrigins
	corsConfig.AllowHeaders = []string{"*"}
	corsConfig.AllowWildcard = false
	corsConfig.AllowCredentials = false
	corsConfig.AddAllowMethods("OPTIONS")
	if config.General.Mode != config.TestMode {
		engine.Use(cors.New(corsConfig))
	}
	if !registered {
		prom := ginprom.New(func(prom *ginprom.Prometheus) {
			prom.Namespace = "gbans"
			prom.Subsystem = "http"
		})
		engine.Use(prom.Instrument())
		registered = true
	}
	staticPath := config.HTTP.StaticPath
	if staticPath == "" {
		staticPath = "./dist"
	}
	absStaticPath, errStaticPath := filepath.Abs(staticPath)
	if errStaticPath != nil {
		log.Fatalf("Invalid static path: %v", errStaticPath)
	}
	// Don't use session for static assets
	// Note that we only use embedded assets for !release modes
	// This is to allow us the ability to develop the frontend without needing to
	// compile+re-embed the assets on each change.
	//if config.General.Mode == config.ReleaseMode {
	//	engine.StaticFS("/dist", http.FS(content))
	//} else {
	//	engine.StaticFS("/dist", http.Dir(absStaticPath))
	//}
	engine.StaticFS("/dist", http.Dir(absStaticPath))
	idxPath := filepath.Join(absStaticPath, "index.html")

	// These should match routes defined in the frontend. This allows us to use the browser
	// based routing when serving the SPA.
	jsRoutes := []string{
		"/", "/servers", "/profile/:steam_id", "/bans", "/appeal", "/settings", "/report",
		"/admin/server_logs", "/admin/servers", "/admin/people", "/admin/ban", "/admin/reports", "/admin/news",
		"/admin/import", "/admin/filters", "/404", "/logout", "/login/success", "/report/:report_id", "/wiki",
		"/wiki/*slug", "/log/:match_id", "/logs", "/ban/:ban_id", "/admin/chat", "/admin/appeals", "/login",
		"/pug", "/quickplay", "/global_stats"}
	for _, rt := range jsRoutes {
		engine.GET(rt, func(c *gin.Context) {
			idx, errRead := os.ReadFile(idxPath)
			if errRead != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				log.Errorf("Failed to load index.html from %s", idxPath)
				return
			}
			c.Data(200, "text/html", idx)
		})
	}
	engine.GET("/auth/callback", web.onOpenIDCallback(database))
	engine.GET("/api/auth/logout", web.onGetLogout())
	engine.POST("/api/auth/refresh", web.onTokenRefresh(database))

	engine.GET("/export/bans/tf2bd", web.onAPIExportBansTF2BD(database))
	engine.GET("/metrics", prometheusHandler())

	engine.GET("/api/profile", web.onAPIProfile(database))
	engine.GET("/api/servers/state", web.onAPIGetServerStates())
	engine.GET("/api/stats", web.onAPIGetStats(database))
	engine.GET("/api/competitive", web.onAPIGetCompHist())

	engine.GET("/api/players", web.onAPIGetPlayers(database))
	engine.GET("/api/wiki/slug/*slug", web.onAPIGetWikiSlug(database))
	engine.GET("/api/log/:match_id", web.onAPIGetMatch(database))
	engine.POST("/api/logs", web.onAPIGetMatches(database))
	engine.GET("/media/:media_id", web.onGetMediaById(database))
	engine.POST("/api/news_latest", web.onAPIGetNewsLatest(database))
	engine.POST("/api/server_query", web.onAPIPostServerQuery(database))
	engine.GET("/api/server_stats", web.onAPIGetTF2Stats(database))

	// Service discovery endpoints
	engine.GET("/api/sd/prometheus/hosts", web.onAPIGetPrometheusHosts(database))
	engine.GET("/api/sd/ansible/hosts", web.onAPIGetPrometheusHosts(database))

	// Game server plugin routes
	engine.POST("/api/server/auth", web.onSAPIPostServerAuth(database))
	engine.POST("/api/resolve_profile", web.onAPIGetResolveProfile(database))

	qpConnections := qpConnectionManager{
		RWMutex:     &sync.RWMutex{},
		lobbies:     map[string]*qpLobby{},
		connections: nil,
	}
	srvGrp := engine.Group("/")
	{
		// Server Auth Request
		serverAuth := srvGrp.Use(web.authServerMiddleWare(database))
		serverAuth.GET("/api/server/admins", web.onAPIGetServerAdmins(database))
		serverAuth.POST("/api/ping_mod", web.onAPIPostPingMod(database))
		serverAuth.POST("/api/check", web.onAPIPostServerCheck(database))
		serverAuth.POST("/api/demo", web.onAPIPostDemo(database))
		serverAuth.POST("/api/log", web.onAPIPostLog(database, logFileC))
		serverAuth.POST("/api/sm/bans/steam/create", web.onAPIPostBanSteamCreate(database))
	}
	authedGrp := engine.Group("/")
	{
		// Basic logged-in user
		authed := authedGrp.Use(authMiddleware(database, model.PUser))
		authed.GET("/ws/quickplay", func(c *gin.Context) {
			currentUser := currentUserProfile(c)
			qpWSHandler(c.Writer, c.Request, &qpConnections, currentUser)
		})

		authed.GET("/api/current_profile", web.onAPICurrentProfile())
		authed.POST("/api/report", web.onAPIPostReportCreate(database))
		authed.GET("/api/report/:report_id", web.onAPIGetReport(database))
		authed.POST("/api/reports", web.onAPIGetReports(database))
		authed.POST("/api/report_status/:report_id", web.onAPISetReportStatus(database))
		authed.POST("/api/media", web.onAPISaveMedia(database))

		authed.GET("/api/report/:report_id/messages", web.onAPIGetReportMessages(database))
		authed.POST("/api/report/:report_id/messages", web.onAPIPostReportMessage(database))
		authed.POST("/api/report/message/:report_message_id", web.onAPIEditReportMessage(database))
		authed.DELETE("/api/report/message/:report_message_id", web.onAPIDeleteReportMessage(database))

		authed.GET("/api/bans/steam/:ban_id", web.onAPIGetBanByID(database))
		authed.GET("/api/bans/:ban_id/messages", web.onAPIGetBanMessages(database))
		authed.POST("/api/bans/:ban_id/messages", web.onAPIPostBanMessage(database))
		authed.POST("/api/bans/message/:ban_message_id", web.onAPIEditBanMessage(database))
		authed.DELETE("/api/bans/message/:ban_message_id", web.onAPIDeleteBanMessage(database))
	}

	editorGrp := engine.Group("/")
	{
		// Editor access
		editorRoute := editorGrp.Use(authMiddleware(database, model.PEditor))
		editorRoute.POST("/api/wiki/slug", web.onAPISaveWikiSlug(database))
		editorRoute.POST("/api/news", web.onAPIPostNewsCreate(database))
		editorRoute.POST("/api/news/:news_id", web.onAPIPostNewsUpdate(database))
		editorRoute.POST("/api/news_all", web.onAPIGetNewsAll(database))
		editorRoute.GET("/api/filters", web.onAPIGetWordFilters(database))
		editorRoute.POST("/api/filters", web.onAPIPostWordFilter(database))
		editorRoute.DELETE("/api/filters/:word_id", web.onAPIDeleteWordFilter(database))
		editorRoute.POST("/api/filter_match", web.onAPIPostWordMatch(database))
	}

	modGrp := engine.Group("/")
	{
		// Moderator access
		modRoute := modGrp.Use(authMiddleware(database, model.PModerator))
		modRoute.POST("/api/report/:report_id/state", web.onAPIPostBanState(database))
		modRoute.GET("/api/connections/:steam_id", web.onAPIGetPersonConnections(database))
		modRoute.GET("/api/messages/:steam_id", web.onAPIGetPersonMessages(database))
		modRoute.GET("/api/message/:person_message_id/context", web.onAPIGetMessageContext(database))
		modRoute.POST("/api/messages", web.onAPIQueryMessages(database))
		modRoute.POST("/api/appeals", web.onAPIGetAppeals(database))
		modRoute.POST("/api/bans/steam", web.onAPIGetBansSteam(database))
		modRoute.POST("/api/bans/steam/create", web.onAPIPostBanSteamCreate(database))
		modRoute.DELETE("/api/bans/steam/:ban_id", web.onAPIPostBanDelete(database))
		modRoute.POST("/api/bans/steam/:ban_id/status", web.onAPIPostSetBanAppealStatus(database))
		modRoute.POST("/api/bans/cidr/create", web.onAPIPostBansCIDRCreate(database))
		modRoute.POST("/api/bans/cidr", web.onAPIGetBansCIDR(database))
		modRoute.DELETE("/api/bans/cidr/:net_id", web.onAPIDeleteBansCIDR(database))
		modRoute.POST("/api/bans/asn/create", web.onAPIPostBansASNCreate(database))
		modRoute.POST("/api/bans/asn", web.onAPIGetBansASN(database))
		modRoute.DELETE("/api/bans/asn/:asn_id", web.onAPIDeleteBansASN(database))
		modRoute.POST("/api/bans/group/create", web.onAPIPostBansGroupCreate(database))
		modRoute.POST("/api/bans/group", web.onAPIGetBansGroup(database))
		modRoute.DELETE("/api/bans/group/:ban_group_id", web.onAPIDeleteBansGroup(database))
	}

	adminGrp := engine.Group("/")
	{
		// Admin access
		adminRoute := adminGrp.Use(authMiddleware(database, model.PAdmin))
		adminRoute.POST("/api/servers", web.onAPIPostServer(database))
		adminRoute.POST("/api/servers/:server_id", web.onAPIPostServerUpdate(database))
		adminRoute.DELETE("/api/servers/:server_id", web.onAPIPostServerDelete(database))
		adminRoute.GET("/api/servers", web.onAPIGetServers(database))
	}
}
