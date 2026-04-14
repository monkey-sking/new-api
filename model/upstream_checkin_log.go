package model

import "github.com/QuantumNous/new-api/common"

const (
	UpstreamCheckinStatusUnchecked = "unchecked"
	UpstreamCheckinStatusSuccess   = "success"
	UpstreamCheckinStatusFailed    = "failed"
	UpstreamCheckinStatusSkipped   = "skipped"
)

type UpstreamCheckinLog struct {
	Id          int              `json:"id"`
	AccountID   int              `json:"account_id" gorm:"not null;index"`
	Account     *UpstreamAccount `json:"account,omitempty" gorm:"foreignKey:AccountID"`
	Status      string           `json:"status" gorm:"size:32;not null;index"`
	Trigger     string           `json:"trigger,omitempty" gorm:"size:32"`
	Message     string           `json:"message,omitempty" gorm:"type:text"`
	Reward      string           `json:"reward,omitempty" gorm:"size:128"`
	CreatedTime int64            `json:"created_time" gorm:"bigint;index"`
}

func (log *UpstreamCheckinLog) BeforeCreate() {
	if log.CreatedTime == 0 {
		log.CreatedTime = common.GetTimestamp()
	}
}

func (log *UpstreamCheckinLog) Insert() error {
	log.BeforeCreate()
	return DB.Create(log).Error
}

func SearchUpstreamCheckinLogs(accountID int, offset int, limit int) ([]*UpstreamCheckinLog, int64, error) {
	var logs []*UpstreamCheckinLog
	db := DB.Model(&UpstreamCheckinLog{}).
		Preload("Account").
		Preload("Account.Site")
	if accountID > 0 {
		db = db.Where("account_id = ?", accountID)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("created_time DESC, id DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}
