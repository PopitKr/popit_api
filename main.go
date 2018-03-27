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

	e := echo.New()

	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(setDbConnContext(xormDb))

	e.GET("/api/RecentPosts", GetRecentPosts)
	e.GET("/api/TagPosts", GetTagPosts)
	e.GET("/api/RandomAuthorPosts", GetRandomAuthorPosts)
	e.GET("/api/PostsByTag", GetPostsByTag)
	e.GET("/api/PostsByAuthor", GetPostsByAuthor)
	e.GET("/api/GetGoogleAd", GetGoogleAd)

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

func GetPostsByAuthor(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, ApiResult{
			Success: false,
			Message: "Wrong id parameter[" + c.QueryParam("id") + "]",
		})
	}

	excludesParam := c.QueryParam("excludes")

	excludes := make([]int, 0)
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

	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		page = 1
	}

	size, err := strconv.Atoi(c.QueryParam("size"))
	if err != nil {
		size = 2
	}

	posts, err := Post{}.GetByAuthor(c.Request().Context(), int64(id), excludes, page, size)
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

func GetPostsByTag(c echo.Context) error {
	tag, err := strconv.Atoi(c.QueryParam("tag"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, ApiResult{
			Success: false,
			Message: "Wrong tag parameter[" + c.QueryParam("tag") + "]",
		})
	}

	excludesParam := c.QueryParam("excludes")

	excludes := make([]int, 0)
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

	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		page = 1
	}

	size, err := strconv.Atoi(c.QueryParam("size"))
	if err != nil {
		size = 2
	}

	posts, err := Post{}.GetByTag(c.Request().Context(), tag, excludes, page, size)
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