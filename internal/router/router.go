package router

import (
	"ledong-db/internal/handler"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func NewRouter(smsHandler *handler.SmsHandler, courseHandler *handler.CourseHandler, efficiencyHandler *handler.EfficiencyHandler, userHandler *handler.UserHandler, cardHandler *handler.CardHandler, pendingCourseHandler *handler.PendingCourseHandler, rechargeNoticeHandler *handler.RechargeNoticeHandler) *gin.Engine {
	r := gin.New()

	r.Use(requestLoggingMiddleware(), recoveryMiddleware(), corsMiddleware())

	r.GET("/health", healthCheck)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// api := r.Group("/api")
	api := r.Group("/api")
	{
		api.GET("/pending-course", pendingCourseHandler.List)
		api.POST("/pending-course/:id/admit", pendingCourseHandler.Admit)
		api.GET("/coach-submissions", rechargeNoticeHandler.ListSubmissions)
		api.GET("/recharge-notices", rechargeNoticeHandler.List)
		api.POST("/recharge-notices/:id/acknowledge", rechargeNoticeHandler.Acknowledge)

		// sms := api.Group("/sms")
		// {
		// 	sms.POST("/notify", smsHandler.Notify)
		// }

		course := api.Group("/course")
		{
			course.GET("/total", courseHandler.TotalCourse)
		}

		coach := api.Group("/coach")
		{
			coach.GET("/efficient", efficiencyHandler.Efficient)
		}

		user := api.Group("/user")
		{
			user.POST("/register", userHandler.Register)
			user.POST("/charged", userHandler.Charged)
			user.GET("/charged/:number", userHandler.GetChargedByNumber)
			user.GET("/charged/total", userHandler.GetChargedTotal)
			user.GET("/charged/coach/:number", userHandler.GetChargedByCoach)
			user.POST("/member/:number", userHandler.SetYonthAndAdult)
			user.POST("/charged/retreat/:id", userHandler.RetreatCharge)
			user.GET("/", userHandler.GetMembers)
			user.POST("/course/notify", userHandler.NotifyCourse)
			user.GET("/coach", userHandler.GetCoaches)
			user.GET("/court", userHandler.GetCourts)
		}

		prepaidCard := api.Group("/prepaidCard")
		{
			prepaidCard.GET("/spend", cardHandler.GetSpend)

			course := prepaidCard.Group("/course")
			{
				course.POST("/create", courseHandler.CreateCourse)
				course.GET("/coach/:coachId", courseHandler.CoachCourses)
				course.DELETE("/:id/:member", courseHandler.RemoveCourseMember)
				course.DELETE("/:id", courseHandler.RemoveCourse)
				course.POST("/trial/:id", courseHandler.TrialCourseUpdate)
				course.GET("/total", courseHandler.TotalCourse)
				course.POST("/notify", courseHandler.NotifyCourse)
				course.POST("/duplicate", courseHandler.DuplicatedCheck)
				course.POST("/sm", courseHandler.CourseSm)
			}

			coach := prepaidCard.Group("/coach")
			{
				coach.GET("/efficient", efficiencyHandler.Efficient)
			}
		}
	}

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, X-Request-ID, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, X-Request-ID, Access-Control-Allow-Origin, Access-Control-Allow-Headers")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// healthCheck 健康检查
// @Summary      健康检查
// @Description  检查服务是否正常运行
// @Tags         系统
// @Success      200  {object}  map[string]string
// @Router       /health [get]
func healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}
