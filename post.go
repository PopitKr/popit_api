package main

import (
	"time"
	"math/rand"
	"unicode/utf8"
	"strings"
	"context"
	"strconv"
	"fmt"
	"golang.org/x/net/html"
	"io/ioutil"
	"encoding/json"
	"net/url"
	"net/http"
	"errors"
)

const (
	MAX_AUTHORS = 5
	MAX_DESCRIPTION_CHARS = 600
)

type Post struct {
	ID int64 					   `json:"id"            xorm:"ID"`
	AuthorID int64       `json:"-"             xorm:"post_author"`
	Author Author  	     `json:"author"        xorm:"-"`
	Content string           `json:"content"       xorm:"post_content"`
	Title string             `json:"title"         xorm:"post_title"`
	PostDate time.Time       `json:"date"          xorm:"post_date"`
	PostName string          `json:"postName"      xorm:"post_name"`
	PostExcerpt string       `json:"-"             xorm:"post_excerpt"`
	Guid string              `json:"-"             xorm:"guid"`
	Image string             `json:"image"         xorm:"-"`
	SocialTitle string       `json:"socialTitle"   xorm:"-"`
	SocialDesc string        `json:"socialDesc"    xorm:"-"`
	Categories []Term        `json:"categories"    xorm:"-"`
	Tags []Term              `json:"tags"          xorm:"-"`
	Metas []PostExternalMeta `json:"metas"         xorm:"-"`
	HighlightedText string   `json:"highlightedText" xorm:"-"`
}

type SearchResult struct {
	TotalHits int `json:"totalHits"`
	Posts     []struct {
		ID              int         `json:"id"`
		HighlightedText string      `json:"highlightedText"`
	} `json:"posts"`
}

func (Post) TableName() (string) {
	return "wprdh0703_posts"
}

type TermPosts struct {
	Term Term    `json:"term"`
	Posts []Post `json:"posts"`
}

type AuthorPosts struct {
	Author Author    `json:"author"`
	Posts []Post     `json:"posts"`
}

type PostMeta struct {
	Key string `xorm:"meta_key"`
	Value string `xorm:"meta_value"`
}

func (PostMeta) TableName() (string) {
	return "wprdh0703_postmeta"
}

func (p Post)Search(ctx context.Context, keyword string, page int) ([]Post, error) {
	encodedKeyword := &url.URL{Path: fmt.Sprintf(`%v`, keyword)}
	searchAPI := fmt.Sprintf(`http://127.0.0.1:8099/api/search/%v?page=%v`, encodedKeyword.String(), page)

	req, err := http.NewRequest("GET", searchAPI, nil)
	if err != nil {
		fmt.Println("ERROR:", err.Error(), " ==>", searchAPI)
		return nil, err
	}
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println("ERROR sending request:", err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		fmt.Println("ERROR wrong status code:", resp.StatusCode, " ==>", searchAPI)
		//return 0, errors.New("wrong status code:" + resp.StatusCode")
		return nil, errors.New("wrong http response status code")
	}
	jsonResult, _ := ioutil.ReadAll(resp.Body)

	var searchResult SearchResult
	err = json.Unmarshal(jsonResult, &searchResult)
	if err != nil {
		fmt.Println("ERROR parsing result:", err.Error())
		fmt.Println("Result:", string(jsonResult))
		return nil, err
	}

	if len(searchResult.Posts) == 0 {
		emptyPosts := make([]Post, 0)
		return emptyPosts, err
	}

	postIds := make([]int64, 0)

	for _, post := range searchResult.Posts {
		postIds = append(postIds, int64(post.ID))
	}

	posts, err := p.GetPostsByIds(ctx, postIds, "post")
	if err != nil {
		return nil, err
	}

	// order by search result
	postMap := make(map[int64]Post)
	for _, post := range posts {
		postMap[post.ID] = post
	}

	orderedPosts := make([]Post, 0)
	for _, searchPost := range searchResult.Posts {
		postId := int64(searchPost.ID)
		post, has := postMap[postId]

		post.HighlightedText = searchPost.HighlightedText

		if has {
			orderedPosts = append(orderedPosts, post)
		}
	}

	return orderedPosts, nil
}

func (Post)GetPostById(ctx context.Context, postId int64) (*Post, error) {
	post := &Post{}

	has, err := GetDBConn(ctx).
		Select("ID, post_author, post_content, post_title, post_date, post_name, guid, post_excerpt").
		Where("post_status = 'draft'").
		And("post_type = 'post'").
		And("ID = ?", postId).
		OrderBy("post_date desc").
		Get(post)

	if !has {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	err = post.loadAssociations(ctx)
	if err != nil {
		return nil, err
	}

	post.processSpecialElement(ctx)
	return post, nil
}

func (Post)GetPostsByIds(ctx context.Context, postIds []int64, postType string) ([]Post, error) {
	var posts []Post

	postStatus := "publish"
	if postType == "attachment" {
		postStatus = "inherit"
	}

	err := GetDBConn(ctx).
		Select("ID, post_author, post_content, post_title, post_date, post_name, guid, post_excerpt").
		Where("post_status = ?", postStatus).
		And("post_type = ?", postType).
		In("ID", postIds).
		Find(&posts)

	if err != nil {
		return nil, err
	}

	return loadPostAssoications(ctx, posts)
}

func (Post)GetByPermalink(ctx context.Context, permalink string) (*Post, error) {
	post := &Post{}

	has, err := GetDBConn(ctx).
		Select("ID, post_author, post_content, post_title, post_date, post_name, guid, post_excerpt").
		Where("post_status = 'publish'").
		And("post_type = 'post'").
		And("post_name = ?", permalink).
		OrderBy("post_date desc").
		Get(post)

	if !has {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	err = post.loadAssociations(ctx)
	if err != nil {
		return nil, err
	}

	post.processSpecialElement(ctx)
	return post, nil
}

func (p *Post) processSpecialElement(ctx context.Context) {
	if !strings.Contains(p.Content, "[gallery") {
		return
	}

	prefix := ""
	processedContent := ""
	lines := strings.Split(p.Content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[gallery") {
			//[gallery columns="2" size="medium" ids="17068,17069"]

			columns := ""
			size := ""
			ids := make([]int64, 0)
			tokens := strings.Split(strings.Replace(strings.Replace(line, "[gallery ", "", -1), "]", "", -1), " ")
			for _, eachToken := range tokens {
				keyVal := strings.Split(eachToken, "=")
				if len(keyVal) != 2 {
					continue
				}
				key := keyVal[0]
				val := keyVal[1]

				if key == "ids" {
					idStr, err := strconv.Unquote(strings.TrimSpace(val))
					if err != nil {
						fmt.Println("gallery id Unquote error:", val, "==>", err.Error())
						return
					}
					for _, id := range strings.Split(idStr, ",") {
						i64, err := strconv.ParseInt(id, 10, 32)
						if err != nil {
							fmt.Println("gallery id error:", err.Error())
							return
						}
						ids = append(ids, i64)
					}
				} else if key == "columns" {
					columns = val
				} else if key == "size" {
					size = val
				}
			}

			childPosts, err := p.GetPostsByIds(ctx, ids, "attachment")
			if err != nil {
				fmt.Println("error while getting child post:", err.Error())
				return
			}

			images := make([]string, 0)
			postExcerpts := make([]string, 0)

			for _, eachId := range ids {
				for _, eachChildPost := range childPosts {
					if eachId == eachChildPost.ID {
						images = append(images, eachChildPost.Guid)
						parameters := url.Values{}
						parameters.Add("", eachChildPost.PostExcerpt)
						encodedPostExcerpt := parameters.Encode()

						postExcerpts = append(postExcerpts, encodedPostExcerpt[1:])
						break
					}
				}
			}
			processedContent += prefix + fmt.Sprintf("[gallery columns=%v size=%v images=\"%v\" captions=\"%v\"]",
				columns, size, strings.Join(images, ","), strings.Join(postExcerpts, ","))
		}	else {
			processedContent += prefix + line
		}
		prefix = "\n"
	}

	p.Content = processedContent
}

func (Post)GetRecent(ctx context.Context, page int, pageSize int) ([]Post, error) {
	var posts []Post

	offset := (page - 1) * pageSize

	err := GetDBConn(ctx).
		Select("ID, post_author, post_content, post_title, post_date, post_name").
		Where("post_status = 'publish'").
		And("post_type = 'post'").
		OrderBy("post_date desc").
		Limit(pageSize, offset).
		Find(&posts)

	if err != nil {
		return nil, err
	}

	return loadPostAssoications(ctx, posts)
}

func loadPostAssoications(ctx context.Context, posts []Post) ([]Post, error) {
	loadedPosts := make([]Post, 0)

	for _, eachPost := range posts {
		if err := (&eachPost).loadAssociations(ctx); err != nil {
			return nil, err
		}

		eachPost.Content = "";	//truncate to reduce size

		loadedPosts = append(loadedPosts, eachPost)
	}
	return loadedPosts, nil
}

func (p *Post) loadAssociations(ctx context.Context) error {
	if err := p.loadAuthor(ctx); err != nil {
		return err
	}

	if err := p.loadMeta(ctx); err != nil {
		return err
	}

	if err := p.loadCategoriesAndTerms(ctx); err != nil {
		return err;
	}

	if extraMetas, err := (PostExternalMeta{}).GetByPost(ctx, p.ID); err != nil {
		return err
	} else {
		p.Metas = extraMetas
	}

	return nil
}

func (p Post)GetRandomPostsByTerm(ctx context.Context, isMobile bool) ([]TermPosts, error) {
	terms, err := Term{}.CountTerm(ctx)
	if err != nil {
		return nil, err
	}

	numTerms := len(terms)

	numResults := MAX_AUTHORS
	if isMobile {
		numResults = 3
	}

	if numTerms < MAX_AUTHORS {
		numResults = numTerms;
	}

	termPostsArray := make([]TermPosts, 0)

	selectedIndexes := make(map[int]bool)
	for i := 0; i < numResults; i++ {
		// find not selected random value
		for {
			termIndex := rand.Intn(numTerms)
			if _, has := selectedIndexes[termIndex]; !has {
				selectedIndexes[termIndex] = true
				break
			}
		}
	}

	pageSize := 5
	if isMobile {
		pageSize = 2
	}

	for index, _ := range selectedIndexes {
		posts, err := p.getTermPosts(ctx, terms[index].Term.ID, "", "RAND()", true,1, pageSize)
		if err != nil {
			return nil, err
		}

		termPosts := 	TermPosts{
			Term: terms[index].Term,
			Posts: posts,
		}
		termPostsArray = append(termPostsArray, termPosts)
	}

	return termPostsArray, nil
}

func (Post)getTermPosts(ctx context.Context, termId int,
	where string, orderBy string, loadAssociation bool, page int, pageSize int) ([]Post, error) {
	var posts []Post

	offset := (page - 1) * pageSize

	query := GetDBConn(ctx).Table("wprdh0703_posts").
		//Select("wprdh0703_posts.*").
		Select("wprdh0703_posts.ID, wprdh0703_posts.post_author, wprdh0703_posts.post_content, wprdh0703_posts.post_title, wprdh0703_posts.post_date, wprdh0703_posts.post_name").
		Join("INNER", "wprdh0703_term_relationships", "wprdh0703_posts.ID = wprdh0703_term_relationships.object_id").
		Join("INNER", "wprdh0703_term_taxonomy", "wprdh0703_term_taxonomy.term_taxonomy_id = wprdh0703_term_relationships.term_taxonomy_id").
		Where("wprdh0703_posts.post_status = 'publish'").
		And("wprdh0703_posts.post_type = 'post'").
		And("wprdh0703_term_taxonomy.term_id = ?", termId)

	if len(where) > 0 {
		query = query.Where(where)
	}

	query = query.OrderBy(orderBy).Limit(pageSize, offset)
	// Run query
	err := query.Find(&posts)
	if err != nil {
		return nil, err
	}

	if loadAssociation {
		loadedPosts, err := loadPostAssoications(ctx, posts)
		if err != nil {
			return nil, err
		}

		return loadedPosts, nil
	} else {
		return posts, nil
	}
}

func (p Post)GetByAuthor(ctx context.Context, authorId int64, excludes []int, page int, pageSize int) ([]Post, error) {
	idList := ""
	prefix := ""
	for _, eachId := range excludes {
		idList += prefix + strconv.Itoa(eachId)
		prefix = ","
	}

	where := ""
	if len(excludes) > 0 {
		where = "wprdh0703_users.ID not in (" + idList + ")"
	}

	posts, err := p.getAuthorPosts(ctx, authorId, where, "wprdh0703_posts.post_date desc", page, pageSize)
	if err != nil {
		return nil, err
	}

	return posts, nil
}

func (p Post)GetRandomPostsByAuthor(ctx context.Context, isMobile bool) ([]AuthorPosts, error) {
	authors, err := Author{}.FindAuthorByPostCount(ctx, 2)
	if err != nil {
		return nil, err
	}

	numAuthors := len(authors)

	numResults := MAX_AUTHORS
	if isMobile {
		numResults = 3
	}
	if numAuthors < MAX_AUTHORS {
		numResults = numAuthors;
	}

	authorPostsArray := make([]AuthorPosts, 0)

	selectedIndexes := make(map[int]bool)
	for i := 0; i < numResults; i++ {
		// find not selected random value
		for {
			authorIndex := rand.Intn(numAuthors)
			if _, has := selectedIndexes[authorIndex]; !has {
				selectedIndexes[authorIndex] = true
				break
			}
		}
	}

	for index, _ := range selectedIndexes {
		posts, err := p.getAuthorPosts(ctx, authors[index].ID, "", "RAND()", 1, 2)
		if err != nil {
			return nil, err
		}
		authorPosts := AuthorPosts{
			Author: authors[index],
			Posts: posts,
		}

		authorPostsArray = append(authorPostsArray, authorPosts)
	}

	return authorPostsArray, nil
}

func (Post)getAuthorPosts(ctx context.Context, authorId int64, where string, orderBy string, page int, pageSize int) ([]Post, error) {
	var posts []Post

	offset := (page - 1) * pageSize

	query := GetDBConn(ctx).Table("wprdh0703_posts").
		Select("wprdh0703_posts.ID, wprdh0703_posts.post_author, wprdh0703_posts.post_content, wprdh0703_posts.post_title, wprdh0703_posts.post_date, wprdh0703_posts.post_name").
		Join("INNER", "wprdh0703_users", "wprdh0703_posts.post_author = wprdh0703_users.ID").
		Where("wprdh0703_posts.post_status = 'publish'").
		And("wprdh0703_posts.post_type = 'post'").
		And("wprdh0703_posts.post_author = ?", authorId)

	if len(where) > 0 {
		query = query.Where(where)
	}

	err := query.OrderBy(orderBy).Limit(pageSize, offset).Find(&posts)

	if err != nil {
		return nil, err
	}

	loadedPosts, err := loadPostAssoications(ctx, posts)
	if err != nil {
		return nil, err
	}

	return loadedPosts, nil
}

func (p Post)GetByTag(ctx context.Context, tagId int, excludeIds []int, page int, pageSize int) ([]Post, error) {
	idList := ""
	prefix := ""
	for _, eachId := range excludeIds {
		idList += prefix + strconv.Itoa(eachId)
		prefix = ","
	}
	where := ""
	if len(excludeIds) > 0 {
		where = "wprdh0703_posts.ID not in (" + idList + ")"
	}

	posts, err := p.getTermPosts(ctx, tagId, where, "wprdh0703_posts.post_date desc", true, page, pageSize)
	if err != nil {
		return nil, err
	}

	return posts, nil
}

func (p *Post)loadAuthor(ctx context.Context) error {
	if author, err := (Author{}).GetOne(ctx, p.AuthorID); err != nil {
		return err
	} else {
		p.Author = *author
	}

	return nil
}

func (p *Post)loadMeta(ctx context.Context) error {
	var postMetas []PostMeta
	err := GetDBConn(ctx).Table("wprdh0703_postmeta").
			Where("post_id = ?", p.ID).
			In("meta_key", "post_image", "_aioseop_description", "_aioseop_title").
			Find(&postMetas)

	if err != nil {
		return err
	}

	for _, eachMeta := range postMetas {
		if eachMeta.Key == "post_image" {
			p.Image = eachMeta.Value
		} else if eachMeta.Key == "_aioseop_description" {
			p.SocialDesc = eachMeta.Value
		} else if eachMeta.Key == "_aioseop_title" {
			p.SocialTitle = eachMeta.Value
		}
	}

	if len(p.SocialTitle) == 0 {
		p.SocialTitle = p.Title
	}

	socialDescText := p.SocialDesc
	if len(socialDescText) == 0 {
		socialDescText = p.getDescriptionFromContents()
		endIndex := 600
		if utf8.RuneCountInString(socialDescText) < endIndex {
			endIndex = utf8.RuneCountInString(socialDescText)
		}

		p.SocialDesc = string([]rune(socialDescText)[:endIndex])

		p.SocialDesc = strings.Replace(p.SocialDesc, "--------------------------", "", -1)
		p.SocialDesc = strings.Replace(p.SocialDesc, "-----------------", "", -1)
		p.SocialDesc = strings.Replace(p.SocialDesc, "*****************", "", -1)
	}
	p.SocialDesc = strings.Replace(p.SocialDesc, "\n", " ", -1)
	p.SocialDesc = html.EscapeString(p.SocialDesc)

	return nil
}

func (p *Post)getDescriptionFromContents() string {
	htmlToken := html.NewTokenizer(strings.NewReader(p.Content))
	socialDescText := ""
	isPreTag := false
	stop := false
	for !stop {
		// token type
		tokenType := htmlToken.Next()
		if tokenType == html.ErrorToken {
			break
		}
		token := htmlToken.Token()
		switch tokenType {
		case html.StartTagToken: // <tag>
			// type Token struct {
			//     Type     TokenType
			//     DataAtom atom.Atom
			//     Data     string
			//     Attr     []Attribute
			// }
			//
			// type Attribute struct {
			//     Namespace, Key, Val string
			// }
			if token.Data == "pre" {
				isPreTag = true
			}
			break
		case html.TextToken: // text between start and end tag
			if !isPreTag {
				socialDescText += strings.TrimSpace(token.Data) + " "
				if len(socialDescText) > MAX_DESCRIPTION_CHARS {
					stop = true
				}
			}
			break
		case html.EndTagToken: // </tag>
			if token.Data == "pre" {
				isPreTag = false
			}
			break
		}
	}

	return strings.TrimSpace(socialDescText)
}

func (p *Post)loadCategoriesAndTerms(ctx context.Context) error {
	var terms []Term

	terms, err := (&Term{}).FindByPost(ctx, p.ID)

	if err != nil {
		return err
	}

	categories := make([]Term, 0)
	tags := make([]Term, 0)
	for _, eachTerm := range terms {
		if eachTerm.Taxonomy == "category" {
			categories = append(categories, eachTerm)
		} else if eachTerm.Taxonomy == "post_tag" {
			tags = append(tags, eachTerm)
		}
	}

	p.Categories = categories
	p.Tags = tags

	return nil
}

func (p *Post)UpdateFacebookLike(ctx context.Context, likes int) error {
	var facebookLike FacebookLike
	has, err := GetDBConn(ctx).Where("post_id = ?", p.ID).Get(&facebookLike)
	if err != nil {
		return err
	}

	if has {
		fmt.Println("Update likes: id=", facebookLike.Id, ", likes=", likes)
		facebookLike.Likes = likes
		_, err := GetDBConn(ctx).Where("id = ?", facebookLike.Id).Update(facebookLike)
		if err != nil {
			return err
		}
	} else {
		fmt.Println("Insert likes: post_id=", p.ID, ", likes=", likes)
		facebookLike.PostId = p.ID
		facebookLike.Likes = likes
		_, err := GetDBConn(ctx).Insert(facebookLike)
		if err != nil {
			return err
		}
	}
	return nil
}

