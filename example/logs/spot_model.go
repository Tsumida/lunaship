package main

import logsv1 "github.com/tsumida/lunaship/example/logs/gen"

type SpotModel struct {
	ID         uint64 `gorm:"column:id"`
	AccountID  uint64 `gorm:"column:account_id"`
	Currency   string `gorm:"column:currency"`
	Deposit    string `gorm:"column:deposit"`
	Frozen     string `gorm:"column:frozen"`
	Version    uint64 `gorm:"column:version"`
	CreateTime uint64 `gorm:"column:create_time"`
	UpdateTime uint64 `gorm:"column:update_time"`
}

func (SpotModel) TableName() string {
	return "spot"
}

func toProtoSpot(row SpotModel) *logsv1.Spot {
	return &logsv1.Spot{
		Id:         row.ID,
		AccountId:  row.AccountID,
		Currency:   row.Currency,
		Deposit:    row.Deposit,
		Frozen:     row.Frozen,
		Version:    row.Version,
		CreateTime: row.CreateTime,
		UpdateTime: row.UpdateTime,
	}
}
