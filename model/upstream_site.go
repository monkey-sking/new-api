package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	UpstreamSiteStatusActive   = "active"
	UpstreamSiteStatusDisabled = "disabled"
)

type UpstreamSite struct {
	Id          int    `json:"id"`
	ChannelID   int    `json:"channel_id,omitempty" gorm:"index"`
	Name        string `json:"name" gorm:"size:128;not null;index"`
	BaseURL     string `json:"base_url" gorm:"size:255;not null"`
	Platform    string `json:"platform" gorm:"size:64;not null;index"`
	Preset      string `json:"preset,omitempty" gorm:"size:64"`
	Proxy       string `json:"proxy,omitempty" gorm:"size:255"`
	Status      string `json:"status" gorm:"size:32;default:'active';index"`
	ExtraConfig string `json:"extra_config,omitempty" gorm:"type:text"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`
}

func (site *UpstreamSite) BeforeCreate() {
	now := common.GetTimestamp()
	if site.CreatedTime == 0 {
		site.CreatedTime = now
	}
	if site.UpdatedTime == 0 {
		site.UpdatedTime = now
	}
	if strings.TrimSpace(site.Status) == "" {
		site.Status = UpstreamSiteStatusActive
	}
}

func (site *UpstreamSite) Insert() error {
	site.BeforeCreate()
	return DB.Create(site).Error
}

func (site *UpstreamSite) Update() error {
	site.UpdatedTime = common.GetTimestamp()
	return DB.Model(&UpstreamSite{}).
		Where("id = ?", site.Id).
		Select("channel_id", "name", "base_url", "platform", "preset", "proxy", "status", "extra_config", "updated_time").
		Updates(site).Error
}

func (site *UpstreamSite) Delete() error {
	return DB.Delete(site).Error
}

func GetUpstreamSiteByID(id int) (*UpstreamSite, error) {
	var site UpstreamSite
	err := DB.First(&site, id).Error
	if err != nil {
		return nil, err
	}
	return &site, nil
}

func SearchUpstreamSites(keyword string, offset int, limit int) ([]*UpstreamSite, int64, error) {
	var sites []*UpstreamSite
	db := DB.Model(&UpstreamSite{})
	if trimmed := strings.TrimSpace(keyword); trimmed != "" {
		like := "%" + trimmed + "%"
		db = db.Where(
			"name LIKE ? OR base_url LIKE ? OR platform LIKE ? OR preset LIKE ?",
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
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&sites).Error; err != nil {
		return nil, 0, err
	}
	return sites, total, nil
}
