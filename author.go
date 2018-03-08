package main

import (
	"errors"
	"fmt"
	"crypto/md5"
)

type Author struct {
	ID int64             `json:"id"           xorm:"ID"`
	UserLogin string     `json:"userLogin"    xorm:"user_login"`
	DisplayName string   `json:"displayName"  xorm:"display_name"`
	UserUrl string       `json:"userUrl"      xorm:"user_url"`
	Avatar string        `json:"avatar"       xorm:"-"`
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

	(&author).initAvatar();

	return &author, nil
}

func (a *Author)initAvatar() {
	hash := md5.Sum([]byte(a.Email))
	a.Avatar = fmt.Sprintf("https://www.gravatar.com/avatar/%x", hash)
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

	for i := 0; i < len(authors); i++ {
		authors[i].initAvatar()
	}
	return authors, nil
}