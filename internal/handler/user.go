package handler

import (
	"net/http"
	"strconv"
	"sync"

	"ledong-db/internal/config"
	"ledong-db/internal/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService *service.UserService
	cardService *service.CardService
	smsService  *service.SmsService
}

func NewUserHandler(userService *service.UserService, cardService *service.CardService, smsService *service.SmsService) *UserHandler {
	return &UserHandler{
		userService: userService,
		cardService: cardService,
		smsService:  smsService,
	}
}

var (
	secretKeyOnce sync.Once
	secretKey     string
)

func verifySecure(c *gin.Context) bool {
	secretKeyOnce.Do(func() {
		cfg := config.Load()
		secretKey = cfg.SecretKey
	})
	secure := c.GetHeader("secure")
	return secure == secretKey
}

// Register 注册用户
// @Summary      注册用户
// @Description  创建新用户
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        secure  header    string  false  "安全验证头"
// @Param        name    formData  string  true   "姓名"
// @Param        number  formData  string  true   "会员编号"
// @Param        court   formData  string  true   "场地"
// @Success      200     {object}  map[string]interface{}
// @Failure      400     {object}  Response
// @Failure      500     {object}  Response
// @Router       /user/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}

	name := c.PostForm("name")
	number := c.PostForm("number")
	court := c.PostForm("court")

	if name == "" || number == "" || court == "" {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "参数不完整"})
		return
	}

	user, err := h.userService.CreateUser(name, number, court)
	if err != nil {
		if err == service.ErrUserExist {
			c.JSON(http.StatusBadRequest, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": "------",
		"user":  user,
	})
}

// Charged 充值
// @Summary      充值
// @Description  为用户充值
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        secure            header    string   false  "安全验证头"
// @Param        number            formData  string   true   "会员编号"
// @Param        charged           formData  float32  false  "充值金额"
// @Param        time              formData  string   false  "充值时间"
// @Param        times             formData  float32  false  "充值次数"
// @Param        annualTimes       formData  float32  false  "年卡次数"
// @Param        annualExpireTime  formData  string   false  "年卡过期时间"
// @Param        worth             formData  int      false  "等值"
// @Param        court             formData  string   true   "场地"
// @Param        coach             formData  string   true   "教练"
// @Param        description       formData  string   false  "备注"
// @Success      200               {object}  model.Charge
// @Failure      400               {object}  Response
// @Failure      500               {object}  Response
// @Router       /user/charged [post]
func (h *UserHandler) Charged(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}

	number := c.PostForm("number")
	court := c.PostForm("court")
	coach := c.PostForm("coach")

	// if number == "" || court == "" || coach == "" {
	if number == "" {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "参数不完整"})
		return
	}

	var charged, times, annualTimes *float32
	if chargedStr := c.PostForm("charged"); chargedStr != "" {
		if v, err := strconv.ParseFloat(chargedStr, 32); err == nil {
			f := float32(v)
			charged = &f
		}
	}
	if timesStr := c.PostForm("times"); timesStr != "" {
		if v, err := strconv.ParseFloat(timesStr, 32); err == nil {
			f := float32(v)
			times = &f
		}
	}
	if annualTimesStr := c.PostForm("annualTimes"); annualTimesStr != "" {
		if v, err := strconv.ParseFloat(annualTimesStr, 32); err == nil {
			f := float32(v)
			annualTimes = &f
		}
	}

	var worth *int
	if worthStr := c.PostForm("worth"); worthStr != "" {
		if v, err := strconv.Atoi(worthStr); err == nil {
			worth = &v
		}
	}

	time := c.PostForm("time")
	annualExpireTime := c.PostForm("annualExpireTime")
	description := c.PostForm("description")

	charge, err := h.cardService.SetRestCharge(number, charged, times, annualTimes, annualExpireTime, worth, court, description, coach, time)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, charge)
}

// GetChargedByNumber 查询某个用户的充值记录
// @Summary      查询某个用户的充值记录
// @Description  根据会员编号查询充值记录
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        number  path      string  true  "会员编号"
// @Success      200     {object}  service.ChargePage
// @Failure      400     {object}  Response
// @Failure      500     {object}  Response
// @Router       /user/charged/{number} [get]
func (h *UserHandler) GetChargedByNumber(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}
	number := c.Param("number")
	if number == "" {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "会员编号不能为空"})
		return
	}

	result, err := h.cardService.GetCharged(number, 1, 50)
	if err != nil {
		if err == service.ErrUserNotFound {
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetChargedTotal 查询所有充值记录
// @Summary      查询所有充值记录
// @Description  查询所有充值记录，支持分页
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        pageSize  query     int  true  "每页数量"
// @Success      200       {object}  service.ChargePage
// @Failure      400       {object}  Response
// @Failure      500       {object}  Response
// @Router       /user/charged/total [get]
func (h *UserHandler) GetChargedTotal(c *gin.Context) {
	pageSizeStr := c.Query("pageSize")
	pageSize := 50
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	result, err := h.cardService.GetChargedTotal(1, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetChargedByCoach 查询教练的充值记录
// @Summary      查询教练的充值记录
// @Description  根据教练编号和时间范围查询充值记录
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        number     path      string  true  "教练编号"
// @Param        startTime  query     string  true  "开始时间"
// @Param        endTime    query     string  true  "结束时间"
// @Success      200        {string}  string
// @Failure      400        {object}  Response
// @Failure      500        {object}  Response
// @Router       /user/charged/coach/{number} [get]
func (h *UserHandler) GetChargedByCoach(c *gin.Context) {
	number := c.Param("number")
	startTime := c.Query("startTime")
	endTime := c.Query("endTime")

	if number == "" || startTime == "" || endTime == "" {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "参数不完整"})
		return
	}

	result, err := h.cardService.GetChargedByCoach(number, startTime, endTime)
	if err != nil {
		if err == service.ErrUserNotFound {
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.String(http.StatusOK, result)
}

// SetYonthAndAdult 设置会员的青少年和成人数量
// @Summary      设置会员的青少年和成人数量
// @Description  设置会员的青少年和成人数量
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        number  path      string  true  "会员编号"
// @Param        yonth   formData  int     true  "青少年数量"
// @Param        adult   formData  int     true  "成人数量"
// @Success      200     {object}  model.PrepaidCard
// @Failure      400     {object}  Response
// @Failure      500     {object}  Response
// @Router       /user/member/{number} [post]
func (h *UserHandler) SetYonthAndAdult(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}

	number := c.Param("number")
	yonthStr := c.PostForm("yonth")
	adultStr := c.PostForm("adult")

	if number == "" || yonthStr == "" || adultStr == "" {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "参数不完整"})
		return
	}

	yonth, err := strconv.Atoi(yonthStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "青少年数量格式错误"})
		return
	}

	adult, err := strconv.Atoi(adultStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "成人数量格式错误"})
		return
	}

	user, err := h.userService.SetYonthAndAdult(number, yonth, adult)
	if err != nil {
		if err == service.ErrUserNotFound {
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// RetreatCharge 退费
// @Summary      退费
// @Description  根据充值记录ID进行退费
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        secure  header    string  false  "安全验证头"
// @Param        id      path      int     true   "充值记录ID"
// @Success      200     {object}  map[string]interface{}
// @Failure      400     {object}  Response
// @Failure      500     {object}  Response
// @Router       /user/charged/retreat/{id} [post]
func (h *UserHandler) RetreatCharge(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "ID格式错误"})
		return
	}

	_, err = h.cardService.RetreatCharge(id)
	if err != nil {
		if err == service.ErrUserNotFound {
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

// GetMembers 查询会员列表或单个会员
// @Summary      查询会员列表或单个会员
// @Description  根据会员编号查询单个会员，或查询所有会员
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        number  query     string  false  "会员编号"
// @Success      200     {object}  model.PrepaidCard
// @Failure      400     {object}  Response
// @Failure      500     {object}  Response
// @Router       /user/ [get]
func (h *UserHandler) GetMembers(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}
	number := c.Query("number")

	if number != "" {
		member, err := h.userService.GetMember(number)
		if err != nil {
			if err == service.ErrUserNotFound {
				c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, member)
		return
	}

	members, err := h.userService.GetMembers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, members)
}

// NotifyCourse 课程通知
// @Summary      课程通知
// @Description  发送课程通知短信
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        secure  header    string  false  "安全验证头"
// @Param        id      formData  int     false  "课程ID，可选，不传则通知所有未通知的课程"
// @Success      200     {object}  Response
// @Failure      400     {object}  Response
// @Failure      500     {object}  Response
// @Router       /user/course/notify [post]
func (h *UserHandler) NotifyCourse(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}

	idStr := c.PostForm("id")
	var id *uint64
	if idStr != "" {
		parsedID, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, Response{Code: 1, Message: "ID格式错误"})
			return
		}
		id = &parsedID
	}

	if err := h.smsService.NotifyAll(id); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Message: "success"})
}

// GetCoaches 查询教练列表
// @Summary      查询教练列表
// @Description  查询所有活跃的教练
// @Tags         用户
// @Accept       json
// @Produce      json
// @Success      200     {array}   model.Coach
// @Failure      500     {object}  Response
// @Router       /user/coach [get]
func (h *UserHandler) GetCoaches(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}
	coaches, err := h.userService.GetCoaches()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, coaches)
}

// GetCourts 查询场地列表
// @Summary      查询场地列表
// @Description  查询所有场地
// @Tags         用户
// @Accept       json
// @Produce      json
// @Success      200     {array}   model.Court
// @Failure      500     {object}  Response
// @Router       /user/court [get]
func (h *UserHandler) GetCourts(c *gin.Context) {
	if !verifySecure(c) {
		c.JSON(http.StatusUnauthorized, Response{Code: 1, Message: "未授权"})
		return
	}
	courts, err := h.userService.GetCourts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, courts)
}
