package main

import (
	"errors"
	"fmt"
)

type Author struct {
	ID int64             `json:"id"           xorm:"ID"`
	UserLogin string     `json:"userLogin"    xorm:"user_login"`
	DisplayName string   `json:"displayName"  xorm:"display_name"`
	UserUrl string       `json:"userUrl"      xorm:"user_url"`
	//Important: Do not include in JSON because of personal data
	Email string         `json:"-"            xorm:"user_email"`
}

func (Author) TableName() (string) {
	return "wprdh0703_users"
}

func (Author) GetOne(id int64) (*Author, error) {
	var author Author

	exists, err := xormDb.Where("ID = ?", id).Get(&author)

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, errors.New("No Author Record")
	}

	return &author, nil
}

func (Author) FindAuthorByPostCount(numPosts int) ([]Author, error) {
	var authors []Author

	sql := fmt.Sprintf(`
		select b.* from (
			SELECT
				post_author,
				count(1) AS cnt
			FROM wprdh0703_posts
			WHERE post_status = 'publish'
						AND post_type = 'post'
			GROUP BY post_author
			HAVING count(1) > %d
		) a
		join wprdh0703_users b on a.post_author = b.ID
		order by b.ID;
  `, numPosts - 1)

  if err := xormDb.SQL(sql).Find(&authors); err != nil {
  	return nil, err
	}

	return authors, nil
}