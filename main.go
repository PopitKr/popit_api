package main

import (
	"github.com/labstack/echo"
	"github.com/go-xorm/xorm"
	_ "github.com/go-sql-driver/mysql"
	"fmt"
	"github.com/labstack/echo/middleware"
	"net/http"
	"log"
	"os"
	"time"
	"context"
	"strconv"
	"strings"
	"net/url"
	"io/ioutil"
	"encoding/json"
)

var (
	xormDb *xorm.Engine
)

func init() {
	dbConn := os.Getenv("DB_CONN")
	if len(dbConn) == 0 {
		dbConn = "root:@tcp(127.0.0.1:3306)/wordpress?charset=utf8&parseTime=True"
	}

	db, err := xorm.NewEngine("mysql", dbConn)
	if err != nil {
		panic(fmt.Errorf("Database open error: %s \n", err))
	}
	db.ShowSQL(false)
	//db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(10 * time.Second)

	xormDb = db

}

type ApiResult struct {
	Data  interface{} 	`json:"data"`
	Success bool        `json:"success"`
	Message string      `json:"message"`
}

func main() {
	defer xormDb.Close()

	//StartGetFacebookLike()

	e := echo.New()

	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(setDbConnContext(xormDb))

	e.GET("/api/Search", SearchPosts)
	e.GET("/api/RecentPosts", GetRecentPosts)
	e.GET("/api/TagPosts", GetTagPosts)
	e.GET("/api/RandomAuthorPosts", GetRandomAuthorPosts)
	e.GET("/api/PostsByTagId", GetPostsByTagId)
	e.GET("/api/PostsByTag", GetPostsByTag)
	e.GET("/api/PostsByCategory", GetPostsByCategory)
	e.GET("/api/PostsByAuthor", GetPostsByAuthor)
	e.GET("/api/PostsByAuthorId", GetPostsByAuthorId)
	e.GET("/api/PostByPermalink", GetPostByPermalink)
	e.GET("/api/GetGoogleAd", GetGoogleAd)
	e.GET("/api/GetSlideShareEmbedLink", GetSlideShareEmbedLink)

	log.Fatal(e.Start(":8000"))
}

func setDbConnContext(xormDb *xorm.Engine) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			session := xormDb.NewSession()
			defer session.Close()

			req := ctx.Request()
			ctx.SetRequest(req.WithContext(context.WithValue(req.Context(), "DB", session)))
			return next(ctx)
		}
	}
}

func GetDBConn(ctx context.Context) *xorm.Session {
	v := ctx.Value("DB")
	if v == nil {
		panic("DB is not exist")
	}
	if db, ok := v.(*xorm.Session); ok {
		return db
	}
	panic("DB is not exist")
}

func SearchPosts(c echo.Context) error {
	keyword := c.QueryParam("keyword")
	if len(keyword) == 0 {
		return c.JSON(http.StatusBadRequest, ApiResult{
			Success: false,
			Message: "No keyword",
		})
	}

	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		page = 1
	}

	posts, err := Post{}.Search(c.Request().Context(), keyword, page)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, ApiResult{
		Success: true,
		Data: posts,
		Message: "",
	})
}

func GetRecentPosts(c echo.Context) error {
	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		page = 1
	}
	size, err := strconv.Atoi(c.QueryParam("size"))
	if err != nil {
		size = 4
	}

	posts, err := Post{}.GetRecent(c.Request().Context(), page, size)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, ApiResult{
		Success: true,
		Data: posts,
		Message: "",
	})
}

func GetTagPosts(c echo.Context) error {
	isMobile := c.QueryParam("isMobile") == "true"
	posts, err := Post{}.GetRandomPostsByTerm(c.Request().Context(), isMobile)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, ApiResult{
		Success: true,
		Data: posts,
		Message: "",
	})
}

func GetRandomAuthorPosts(c echo.Context) error {
	isMobile := c.QueryParam("isMobile") == "true"

	posts, err := Post{}.GetRandomPostsByAuthor(c.Request().Context(), isMobile)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, ApiResult{
		Success: true,
		Data: posts,
		Message: "",
	})
}

func GetPostsByAuthorId(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, ApiResult{
			Success: false,
			Message: "Wrong id parameter[" + c.QueryParam("id") + "]",
		})
	}

	author, err := Author{}.GetOne(c.Request().Context(), int64(id))

	if author == nil {
		return c.JSON(http.StatusNotFound, ApiResult{
			Success: false,
			Message: "Author not found",
		})
	}

	if err != nil {
		return c.JSON(http.StatusBadRequest, ApiResult{
			Success: false,
			Message: "Wrong author parameter[" + c.QueryParam("author") + "]",
		})
	}

	return getPostsByAuthor(c, author)
}

func GetPostsByAuthor(c echo.Context) error {
	loginName := c.QueryParam("author")

	author, err := Author{}.GetByLoginName(c.Request().Context(), loginName)

	if author == nil {
		return c.JSON(http.StatusNotFound, ApiResult{
			Success: false,
			Message: "Author " + loginName + " not found",
		})
	}

	if err != nil {
		return c.JSON(http.StatusBadRequest, ApiResult{
			Success: false,
			Message: "Wrong author parameter[" + c.QueryParam("author") + "]",
		})
	}

	return getPostsByAuthor(c, author)
}

func getPostsByAuthor(c echo.Context, author *Author) error {
	excludesParam := c.QueryParam("excludes")

	excludes := make([]int, 0)
	if len(strings.TrimSpace(excludesParam)) > 0 {
		for _, eachIdStr := range strings.Split(excludesParam, ",") {
			id, err := strconv.Atoi(eachIdStr)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ApiResult{
					Success: false,
					Message: "Wrong exclude post id: " + excludesParam,
				})
			}
			excludes = append(excludes, id)
		}
	}

	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		page = 1
	}

	size, err := strconv.Atoi(c.QueryParam("size"))
	if err != nil {
		size = 2
	}

	posts, err := Post{}.GetByAuthor(c.Request().Context(), int64(author.ID), excludes, page, size)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	}

	type AuthorPosts struct {
		Author Author	`json:"author"`
		Posts []Post	`json:"posts"`
	}

	return c.JSON(http.StatusOK, ApiResult{
		Success: true,
		Data: AuthorPosts {
			Author: *author,
			Posts: posts,
		},
		Message: "",
	})
}

func GetGoogleAd(c echo.Context) error {
	mode := c.QueryParam("mode")
	ads := make(map[string]SitePreference)

	adKey := "ad." + mode + ".top";
	if ad, err := (SitePreference{}).GetByName(c.Request().Context(), adKey); err != nil {
		return c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	} else if ad != nil {
		ads[adKey] = *ad
	}

	adKey = "ad." + mode + ".middle";
	if ad, err := (SitePreference{}).GetByName(c.Request().Context(), adKey); err != nil {
		return c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	} else if ad != nil {
		ads[adKey] = *ad
	}

	adKey = "ad." + mode + ".bottom";
	if ad, err := (SitePreference{}).GetByName(c.Request().Context(), adKey); err != nil {
		return c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	} else if ad != nil {
		ads[adKey] = *ad
	}

	return c.JSON(http.StatusOK, ApiResult{
		Success: true,
		Data: ads,
		Message: "",
	})
}

func GetPostByPermalink(c echo.Context) error {
	permalink := c.QueryParam("permalink")
	permalink = url.QueryEscape(permalink)
	if len(permalink) == 0 {
		return c.JSON(http.StatusBadRequest, ApiResult{
			Success: false,
			Message: "Wrong permalink parameter[" + c.QueryParam("permalink") + "]",
		})
	}
	post, err := Post{}.GetByPermalink(c.Request().Context(), permalink)

	if post == nil {
		return c.JSON(http.StatusNotFound, ApiResult{
			Success: false,
			Message: permalink + " Not Found",
		})
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, ApiResult{
		Success: true,
		Data: post,
		Message: "",
	})
}

func GetPostsByCategory(c echo.Context) error {
	category := c.QueryParam("category")
	if len(category) == 0 {
		return c.JSON(http.StatusBadRequest, ApiResult{
			Success: false,
			Message: "Wrong tag parameter[" + c.QueryParam("category") + "]",
		})
	}

	category = url.QueryEscape(category);

	term, err := Term{}.FinyBySlug(c.Request().Context(), category, "category")

	if term == nil {
		return c.JSON(http.StatusNotFound, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	}

	return getPostsByTagId(c, term.ID)
}

func GetPostsByTag(c echo.Context) error {
	tag := c.QueryParam("tag")
	if len(tag) == 0 {
		return c.JSON(http.StatusBadRequest, ApiResult{
			Success: false,
			Message: "Wrong tag parameter[" + c.QueryParam("tag") + "]",
		})
	}

	tag = url.QueryEscape(tag);

	term, err := Term{}.FinyBySlug(c.Request().Context(), tag, "post_tag")

	if term == nil {
		return c.JSON(http.StatusNotFound, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	}

	return getPostsByTagId(c, term.ID)
}

func GetPostsByTagId(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, ApiResult{
			Success: false,
			Message: "Wrong id parameter[" + c.QueryParam("id") + "]",
		})
	}

	return getPostsByTagId(c, id)
}

func getPostsByTagId(c echo.Context, id int) error {
	excludesParam := c.QueryParam("excludes")

	excludes := make([]int, 0)
	if len(excludesParam) > 0 {
		for _, eachIdStr := range strings.Split(excludesParam, ",") {
			id, err := strconv.Atoi(eachIdStr)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ApiResult{
					Success: false,
					Message: "Wrong exclude post id: " + excludesParam,
				})
			}
			excludes = append(excludes, id)
		}
	}

	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		page = 1
	}

	size, err := strconv.Atoi(c.QueryParam("size"))
	if err != nil {
		size = 2
	}

	posts, err := Post{}.GetByTag(c.Request().Context(), id, excludes, page, size)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, ApiResult{
		Success: true,
		Data: posts,
		Message: "",
	})
}

func GetSlideShareEmbedLink(c echo.Context) error {
	link := c.QueryParam("link")

	slideshareApi := fmt.Sprintf(`https://www.slideshare.net/api/oembed/2?url=%v&format=json`, link)
	req, err := http.NewRequest("GET", slideshareApi, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	}
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println("ERROR sending request:", err.Error())
		c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: err.Error(),
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		fmt.Println("ERROR wrong status code:", resp.StatusCode, " ==>", slideshareApi)
		c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: fmt.Sprintf("wrong http response status code: %s", resp.StatusCode),
		})
	}
	jsonResult, _ := ioutil.ReadAll(resp.Body)

	jsonMap := make(map[string]interface{})
	err = json.Unmarshal(jsonResult, &jsonMap)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiResult{
			Success: false,
			Message: fmt.Sprintf("wrong http response status code: %s", resp.StatusCode),
		})
	}
	return c.JSON(http.StatusOK, ApiResult{
		Success: true,
		Data: jsonMap,
		Message: "",
	})
}