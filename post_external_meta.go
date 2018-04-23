package main

import (
	"fmt"
	"context"
	"net/http"
	"time"
	"io/ioutil"
	"encoding/json"
	"github.com/pkg/errors"
)

type PostExternalMeta struct {
	Id int64 `json:"id" xorm:"id"`
	PostId int64 `json:"postId" xorm:"post_id"`
	Name  string `json:"name" xorm:"name"`
	Value string `json:"value" xorm:"value"`
}

func (PostExternalMeta) TableName() (string) {
	return "post_external_metas"
}

func (PostExternalMeta) GetByPost(ctx context.Context, postId int64) ([]PostExternalMeta, error) {
	var metas []PostExternalMeta

	err := GetDBConn(ctx).Where("post_id = ?", postId).Find(&metas)

	if err != nil {
		return nil, err
	}

	return metas, nil
}

// Getting Facebook Like
type FacebookLike struct {
	Id int64 `json:"id" xorm:"id"`
	PostId int64 `json:"postId" xorm:"post_id"`
	Likes  int `json:"likes" xorm:"likes"`
}

type Values struct {
	m map[string]interface{}
}

func dumpMap(space string, m map[string]interface{}) {
	for k, v := range m {
		if mv, ok := v.(map[string]interface{}); ok {
			fmt.Printf("{ \"%v\": \n", k)
			dumpMap(space+"\t", mv)
			fmt.Printf("}\n")
		} else {
			fmt.Printf("%v %v : %v\n", space, k, v)
		}
	}
}

func StartGetFacebookLike() {
	go func() {
		for {
			time.Sleep(2 * time.Second)

			session := xormDb.NewSession()
			defer session.Close()

			ctx := context.WithValue(context.Background(), "DB", session)

			posts, err := Post{}.GetRecent(ctx, 1, 100000)
			if err != nil {
				fmt.Println("ERROR:", err.Error())
				continue
			}
			for _, post := range posts {
				time.Sleep(20 * time.Second)
				httpShareCount, err := getFacebookLike(ctx, post, "http")
				if err != nil {
					continue
				}
				httpsShareCount, err := getFacebookLike(ctx, post, "https")
				if err != nil {
					continue
				}

				shareCount := httpShareCount
				if httpsShareCount  > shareCount {
					shareCount = httpsShareCount
				}

				if shareCount == 0 {
					fmt.Println("==> Like is zero:", post.PostName)
					continue;
				}

				err = post.UpdateFacebookLike(ctx, shareCount)
				if err != nil {
					fmt.Println("ERROR:", err.Error())
				}
			}

			time.Sleep(10 * time.Hour)
		}
	}()
}
func getFacebookLike(ctx context.Context, post Post, protocol string) (int, error){
	facebookAPI := "https://graph.facebook.com/?ids="
	postLink := post.PostName
	if postLink[len(postLink) - 1] != '/' {
		postLink = postLink + "/"
	}
	postAPI := fmt.Sprintf(`%v%v://www.popit.kr/%v`, facebookAPI, protocol, postLink)
	req, err := http.NewRequest("GET", postAPI, nil)
	if err != nil {
		fmt.Println("ERROR:", err.Error(), " ==>", postAPI)
		return 0, err
	}
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println("ERROR sending request:", err.Error())
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		fmt.Println("ERROR wrong status code:", resp.StatusCode, " ==>", postAPI)
		//return 0, errors.New("wrong status code:" + resp.StatusCode")
		return 0, errors.New("wrong http response status code")
	}
	jsonResult, _ := ioutil.ReadAll(resp.Body)

	jsonMap := make(map[string]interface{})
	err = json.Unmarshal(jsonResult, &jsonMap)
	if err != nil {
		fmt.Println("ERROR parsing result:", err.Error())
		fmt.Println("Result:", string(jsonResult))
		return 0, err
	}
	dumpMap("", jsonMap)

	shareCount := 0
	for _, v := range jsonMap {
		fmt.Println(">>>>>v:", v)
		valueMap := v.(map[string]interface{})
		share := valueMap["share"]
		shareMap := share.(map[string]interface{})
		shareCountIf := shareMap["share_count"]
		shareCount = int(shareCountIf.(float64))
		break
	}

	return shareCount, nil
}