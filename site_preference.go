package main

import (
	"context"
)

type SitePreference struct {
	ID int `json:"id"           xorm:"id"`
  Name string `json:"name"    xorm:"name"`
  Value string `json:"value"  xorm:"value"`
}

func (SitePreference) TableName() string {
	return "site_prefs"
}

func (SitePreference)GetByName(ctx context.Context, name string) (*SitePreference, error) {
	sitePref := SitePreference{Name: name}

	has, err := GetDBConn(ctx).Get(&sitePref)

	if err != nil {
		return nil, err
	}

	if !has {
		return nil, nil
	}

	return &sitePref, nil
}