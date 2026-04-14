package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	UpstreamAccountStatusActive   = "active"
	UpstreamAccountStatusDisabled = "disabled"
	UpstreamAccountStatusExpired  = "expired"
)

type UpstreamAccount struct {
	Id                   int           `json:"id"`
	SiteID               int           `json:"site_id" gorm:"not null;index"`
	Site                 *UpstreamSite `json:"site,omitempty" gorm:"foreignKey:SiteID"`
	Name                 string        `json:"name" gorm:"size:128;not null;index"`
	Username             string        `json:"username,omitempty" gorm:"size:128;index"`
	Password             string        `json:"-" gorm:"type:text"`
	AccessToken          string        `json:"access_token,omitempty" gorm:"type:text"`
	APIToken             string        `json:"api_token,omitempty" gorm:"type:text"`
	PlatformUserID       int           `json:"platform_user_id,omitempty" gorm:"index"`
	CheckinEnabled       bool          `json:"checkin_enabled"`
	CheckinIntervalHours int           `json:"checkin_interval_hours"`
	LastCheckinAt        int64         `json:"last_checkin_at,omitempty" gorm:"bigint"`
	LastCheckinStatus    string        `json:"last_checkin_status,omitempty" gorm:"size:32"`
	LastCheckinMessage   string        `json:"last_checkin_message,omitempty" gorm:"type:text"`
	LastCheckinReward    string        `json:"last_checkin_reward,omitempty" gorm:"size:128"`
	Status               string        `json:"status" gorm:"size:32;default:'active';index"`
	ExtraConfig          string        `json:"extra_config,omitempty" gorm:"type:text"`
	CreatedTime          int64         `json:"created_time" gorm:"bigint"`
	UpdatedTime          int64         `json:"updated_time" gorm:"bigint"`
}

func (account *UpstreamAccount) BeforeCreate() {
	now := common.GetTimestamp()
	if account.CreatedTime == 0 {
		account.CreatedTime = now
	}
	if account.UpdatedTime == 0 {
		account.UpdatedTime = now
	}
	if strings.TrimSpace(account.Status) == "" {
		account.Status = UpstreamAccountStatusActive
	}
	if account.CheckinIntervalHours <= 0 {
		account.CheckinIntervalHours = 24
	}
}

func (account *UpstreamAccount) Insert() error {
	account.BeforeCreate()
	return DB.Create(account).Error
}

func (account *UpstreamAccount) Update() error {
	account.UpdatedTime = common.GetTimestamp()
	if account.CheckinIntervalHours <= 0 {
		account.CheckinIntervalHours = 24
	}
	return DB.Model(&UpstreamAccount{}).
		Where("id = ?", account.Id).
		Select(
			"site_id",
			"name",
			"username",
			"password",
			"access_token",
			"api_token",
			"platform_user_id",
			"checkin_enabled",
			"checkin_interval_hours",
			"last_checkin_at",
			"last_checkin_status",
			"last_checkin_message",
			"last_checkin_reward",
			"status",
			"extra_config",
			"updated_time",
		).
		Updates(account).Error
}

func (account *UpstreamAccount) Delete() error {
	return DB.Delete(account).Error
}

func GetUpstreamAccountByID(id int) (*UpstreamAccount, error) {
	var account UpstreamAccount
	err := DB.Preload("Site").First(&account, id).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func SearchUpstreamAccounts(keyword string, siteID int, offset int, limit int) ([]*UpstreamAccount, int64, error) {
	var accounts []*UpstreamAccount
	db := DB.Model(&UpstreamAccount{}).Preload("Site")
	if siteID > 0 {
		db = db.Where("site_id = ?", siteID)
	}
	if trimmed := strings.TrimSpace(keyword); trimmed != "" {
		like := "%" + trimmed + "%"
		db = db.Joins("LEFT JOIN upstream_sites ON upstream_sites.id = upstream_accounts.site_id").
			Where(
				"upstream_accounts.name LIKE ? OR upstream_accounts.username LIKE ? OR upstream_accounts.status LIKE ? OR upstream_sites.name LIKE ? OR upstream_sites.base_url LIKE ?",
				like,
				like,
				like,
				like,
				like,
			)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("upstream_accounts.id DESC").Offset(offset).Limit(limit).Find(&accounts).Error; err != nil {
		return nil, 0, err
	}
	return accounts, total, nil
}
