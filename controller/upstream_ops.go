package controller

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	upstreamAccountCheckinTaskDefaultIntervalMinutes = 30
	upstreamAccountCheckinTaskBatchSize              = 100
)

var (
	upstreamAccountCheckinTaskOnce    sync.Once
	upstreamAccountCheckinTaskRunning atomic.Bool
)

type upstreamSiteRequest struct {
	Id        int    `json:"id"`
	ChannelID int    `json:"channel_id"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	Platform  string `json:"platform"`
	Preset    string `json:"preset"`
	Proxy     string `json:"proxy"`
	Status    string `json:"status"`
}

type upstreamAccountRequest struct {
	Id                   int    `json:"id"`
	SiteID               int    `json:"site_id"`
	Name                 string `json:"name"`
	Username             string `json:"username"`
	Password             string `json:"password"`
	AccessToken          string `json:"access_token"`
	CheckinEnabled       bool   `json:"checkin_enabled"`
	CheckinIntervalHours int    `json:"checkin_interval_hours"`
	Status               string `json:"status"`
}

func GetUpstreamSites(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	sites, total, err := model.SearchUpstreamSites(c.Query("keyword"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(sites)
	common.ApiSuccess(c, pageInfo)
}

func CreateUpstreamSite(c *gin.Context) {
	var req upstreamSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	site := &model.UpstreamSite{
		ChannelID: req.ChannelID,
		Name:      req.Name,
		BaseURL:   req.BaseURL,
		Platform:  req.Platform,
		Preset:    req.Preset,
		Proxy:     req.Proxy,
		Status:    req.Status,
	}
	if err := service.NormalizeUpstreamSite(site); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := site.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, site)
}

func UpdateUpstreamSite(c *gin.Context) {
	var req upstreamSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Id <= 0 {
		common.ApiErrorMsg(c, "缺少站点 ID")
		return
	}
	site, err := model.GetUpstreamSiteByID(req.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	site.ChannelID = req.ChannelID
	site.Name = req.Name
	site.BaseURL = req.BaseURL
	site.Platform = req.Platform
	site.Preset = req.Preset
	site.Proxy = req.Proxy
	site.Status = req.Status
	if err := service.NormalizeUpstreamSite(site); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := site.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, site)
}

func DeleteUpstreamSite(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	site, err := model.GetUpstreamSiteByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := site.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func GetUpstreamAccounts(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	siteID, _ := strconv.Atoi(c.Query("site_id"))
	accounts, total, err := model.SearchUpstreamAccounts(c.Query("keyword"), siteID, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(accounts)
	common.ApiSuccess(c, pageInfo)
}

func CreateUpstreamAccount(c *gin.Context) {
	var req upstreamAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	account := &model.UpstreamAccount{
		SiteID:               req.SiteID,
		Name:                 req.Name,
		Username:             req.Username,
		Password:             req.Password,
		AccessToken:          req.AccessToken,
		CheckinEnabled:       req.CheckinEnabled,
		CheckinIntervalHours: req.CheckinIntervalHours,
		Status:               req.Status,
	}
	if err := service.ValidateUpstreamAccount(account); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := account.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	reloaded, err := model.GetUpstreamAccountByID(account.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, reloaded)
}

func UpdateUpstreamAccount(c *gin.Context) {
	var req upstreamAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Id <= 0 {
		common.ApiErrorMsg(c, "缺少账号 ID")
		return
	}
	account, err := model.GetUpstreamAccountByID(req.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	account.SiteID = req.SiteID
	account.Name = req.Name
	account.Username = req.Username
	if strings.TrimSpace(req.Password) != "" {
		account.Password = req.Password
	}
	if strings.TrimSpace(req.AccessToken) != "" {
		account.AccessToken = req.AccessToken
	}
	account.CheckinEnabled = req.CheckinEnabled
	account.CheckinIntervalHours = req.CheckinIntervalHours
	account.Status = req.Status
	if err := service.ValidateUpstreamAccount(account); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := account.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	reloaded, err := model.GetUpstreamAccountByID(account.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, reloaded)
}

func DeleteUpstreamAccount(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	account, err := model.GetUpstreamAccountByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := account.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func RefreshUpstreamAccountSession(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	account, err := model.GetUpstreamAccountByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	session, err := service.RefreshUpstreamAccountSession(account)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	reloaded, err := model.GetUpstreamAccountByID(account.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"account": reloaded,
		"session": session,
	})
}

func RunUpstreamAccountCheckin(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	account, err := model.GetUpstreamAccountByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.RunUpstreamAccountCheckin(account, "manual")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetUpstreamCheckinLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	accountID, _ := strconv.Atoi(c.Query("account_id"))
	logs, total, err := model.SearchUpstreamCheckinLogs(accountID, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

func StartUpstreamAccountCheckinTask() {
	upstreamAccountCheckinTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		if !common.GetEnvOrDefaultBool("UPSTREAM_ACCOUNT_CHECKIN_TASK_ENABLED", true) {
			common.SysLog("upstream account checkin task disabled by UPSTREAM_ACCOUNT_CHECKIN_TASK_ENABLED")
			return
		}

		intervalMinutes := common.GetEnvOrDefault(
			"UPSTREAM_ACCOUNT_CHECKIN_TASK_INTERVAL_MINUTES",
			upstreamAccountCheckinTaskDefaultIntervalMinutes,
		)
		if intervalMinutes < 1 {
			intervalMinutes = upstreamAccountCheckinTaskDefaultIntervalMinutes
		}
		interval := time.Duration(intervalMinutes) * time.Minute

		go func() {
			common.SysLog(fmt.Sprintf("upstream account checkin task started: interval=%s", interval))
			runUpstreamAccountCheckinTaskOnce()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for range ticker.C {
				runUpstreamAccountCheckinTaskOnce()
			}
		}()
	})
}

func runUpstreamAccountCheckinTaskOnce() {
	if !upstreamAccountCheckinTaskRunning.CompareAndSwap(false, true) {
		return
	}
	defer upstreamAccountCheckinTaskRunning.Store(false)

	now := common.GetTimestamp()
	lastID := 0

	for {
		var accounts []*model.UpstreamAccount
		err := model.DB.
			Preload("Site").
			Where("id > ?", lastID).
			Where("status <> ?", model.UpstreamAccountStatusDisabled).
			Order("id asc").
			Limit(upstreamAccountCheckinTaskBatchSize).
			Find(&accounts).Error
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to load upstream accounts: %v", err))
			return
		}
		if len(accounts) == 0 {
			return
		}

		for _, account := range accounts {
			lastID = account.Id
			if !account.CheckinEnabled {
				continue
			}
			intervalHours := account.CheckinIntervalHours
			if intervalHours <= 0 {
				intervalHours = 24
			}
			if account.LastCheckinAt > 0 && now-account.LastCheckinAt < int64(intervalHours)*3600 {
				continue
			}
			if _, err := service.RunUpstreamAccountCheckin(account, "auto"); err != nil {
				common.SysLog(fmt.Sprintf("failed to run upstream account checkin: account_id=%d, error=%v", account.Id, err))
			}
		}

		if len(accounts) < upstreamAccountCheckinTaskBatchSize {
			return
		}
	}
}
