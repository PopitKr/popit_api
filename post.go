package main

import (
	"time"
	"math/rand"
	"github.com/jaytaylor/html2text"
	"unicode/utf8"
	"strings"
	"context"
	"strconv"
)

const (
	MAX_AUTHORS = 5
)

type Post struct {
	ID int64 					   `json:"id"            xorm:"ID"`
	AuthorID int64       `json:"-"             xorm:"post_author"`
	Author Author  	     `json:"author"        xorm:"-"`
	Content string       `json:"content"       xorm:"post_content"`
	Title string		     `json:"title"         xorm:"post_title"`
	PostDate time.Time   `json:"date"          xorm:"post_date"`
	PostName string      `json:"postName"      xrom:"post_name"`
	Image string         `json:"image"         xorm:"-"`
	SocialTitle string   `json:"socialTitle"   xorm:"-"`
	SocialDesc string    `json:"socialDesc"    xorm:"-"`
	Categories []Term    `json:"categories"    xorm:"-"`
	Tags []Term          `json:"tags"          xorm:"-"`
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

func (Post)GetRecent(ctx context.Context, page int, pageSize int) ([]Post, error) {
	var posts []Post

	offset := (page - 1) * pageSize

	err := GetDBConn(ctx).
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
		if err := (&eachPost).loadAssoications(ctx); err != nil {
			return nil, err
		}

		loadedPosts = append(loadedPosts, eachPost)
	}
	return loadedPosts, nil
}

func (p *Post) loadAssoications(ctx context.Context) error {
	if err := p.loadAuthor(ctx); err != nil {
		return err
	}

	if err := p.loadMeta(ctx); err != nil {
		return err
	}
	if err := p.loadCategoriesAndTerms(ctx); err != nil {
		return err;
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
		Select("wprdh0703_posts.*").
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
		Select("wprdh0703_posts.*").
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

		if len(p.SocialTitle) == 0 {
			p.SocialTitle = p.Title
		}

		socialDescText := p.SocialDesc
		if len(socialDescText) == 0 {
			socialDescText, _ = html2text.FromString(p.Content, html2text.Options{PrettyTables: false})
		}
		endIndex := 150
		if utf8.RuneCountInString(socialDescText) < endIndex {
			endIndex = utf8.RuneCountInString(socialDescText)
		}

		p.SocialDesc = string([]rune(socialDescText)[:endIndex])

		p.SocialDesc = strings.Replace(p.SocialDesc, "--------------------------", "", -1)
		p.SocialDesc = strings.Replace(p.SocialDesc, "-----------------", "", -1)
		p.SocialDesc = strings.Replace(p.SocialDesc, "*****************", "", -1)
	}

	return nil
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


