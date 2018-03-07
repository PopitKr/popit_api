package main

import (
	"time"
	"math/rand"
)

type Post struct {
	ID int64 					   `json:"id"            xorm:"ID"`
	AuthorID int64       `json:"-"             xorm:"post_author"`
	Author Author  	     `json:"author"        xorm:"-"`
	Content string       `json:"content"       xorm:"post_content"`
	Title string		     `json:"title"         xorm:"post_title"`
	PostDate time.Time   `json:"date"          xorm:"post_date"`
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

func (Post)GetRecent() ([]Post, error) {
	var posts []Post
	err := xormDb.
		Where("post_status = 'publish'").
		And("post_type = 'post'").
		OrderBy("post_date desc").
		Limit(5).
		Find(&posts)

	if err != nil {
		return nil, err
	}

	return loadPostAssoications(posts)
}

func loadPostAssoications(posts []Post) ([]Post, error) {
	loadedPosts := make([]Post, 0)

	for _, eachPost := range posts {
		if err := eachPost.loadAuthor(); err != nil {
			return nil, err
		}

		if err := eachPost.loadMeta(); err != nil {
			return nil, err
		}
		if err := eachPost.loadCategoriesAndTerms(); err != nil {
			return nil, err;
		}

		loadedPosts = append(loadedPosts, eachPost)
	}
	return loadedPosts, nil
}

func (p Post)GetRandomPostsByTerm() ([]TermPosts, error) {
	terms, err := Term{}.CountTerm()
	if err != nil {
		return nil, err
	}

	numTerms := len(terms)

	numResults := 3
	if numTerms < 3 {
		numResults = numTerms;
	}

	termPostsArray := make([]TermPosts, 0)

	selectedIndexes := []int{-1, -1, -1}
	for i := 0; i < numResults; i++ {
		termIndex := -1
		for {
			// find not selected random value
			termIndex = rand.Intn(numTerms)
			if i == 0 {
				break;
			}
			found := false
			for j := 0; j < i; j++ {
				if termIndex != selectedIndexes[j] {
					found = true
					break;
				}
			}
			if found {
				break;
			}
		}
		selectedIndexes[i] = termIndex
		termPosts, err := p.GetTermPosts(terms[termIndex].Term)
		if err != nil {
			return nil, err
		}
		termPostsArray = append(termPostsArray, *termPosts)
	}

	return termPostsArray, nil
}

func (Post)GetTermPosts(term Term) (*TermPosts, error) {
	var posts []Post

	err := xormDb.Table("wprdh0703_posts").
		Select("wprdh0703_posts.*").
		Join("INNER", "wprdh0703_term_relationships", "wprdh0703_posts.ID = wprdh0703_term_relationships.object_id").
		Join("INNER", "wprdh0703_term_taxonomy", "wprdh0703_term_taxonomy.term_taxonomy_id = wprdh0703_term_relationships.term_taxonomy_id").
		Where("wprdh0703_posts.post_status = 'publish'").
		And("wprdh0703_posts.post_type = 'post'").
		And("wprdh0703_term_taxonomy.term_id = ?", term.ID).
		OrderBy("RAND()").
		Limit(5).
		Find(&posts)

	if err != nil {
		return nil, err
	}

	loadedPosts, err := loadPostAssoications(posts)
	return &TermPosts{
		Term: term,
		Posts: loadedPosts,
	}, nil
}

func (p Post)GetRandomPostsByAuthor() ([]AuthorPosts, error) {
	authors, err := Author{}.FindAuthorByPostCount(2)
	if err != nil {
		return nil, err
	}

	numAuthors := len(authors)

	numResults := 3
	if numAuthors < 3 {
		numResults = numAuthors;
	}

	authorPostsArray := make([]AuthorPosts, 0)

	selectedIndexes := []int{-1, -1, -1}
	for i := 0; i < numResults; i++ {
		authorIndex := -1
		for {
			// find not selected random value
			authorIndex = rand.Intn(numAuthors)
			if i == 0 {
				break;
			}
			found := false
			for j := 0; j < i; j++ {
				if authorIndex != selectedIndexes[j] {
					found = true
					break;
				}
			}
			if found {
				break;
			}
		}
		selectedIndexes[i] = authorIndex
		termPosts, err := p.GetAuthorPosts(authors[authorIndex])
		if err != nil {
			return nil, err
		}
		authorPostsArray = append(authorPostsArray, *termPosts)
	}

	return authorPostsArray, nil
}

func (Post)GetAuthorPosts(author Author) (*AuthorPosts, error) {
	var posts []Post

	err := xormDb.Table("wprdh0703_posts").
		Select("wprdh0703_posts.*").
		Join("INNER", "wprdh0703_users", "wprdh0703_posts.post_author = wprdh0703_users.ID").
		Where("wprdh0703_posts.post_status = 'publish'").
		And("wprdh0703_posts.post_type = 'post'").
		And("wprdh0703_posts.post_author = ?", author.ID).
		OrderBy("RAND()").
		Limit(5).
		Find(&posts)

	if err != nil {
		return nil, err
	}

	loadedPosts, err := loadPostAssoications(posts)
	return &AuthorPosts{
		Author: author,
		Posts: loadedPosts,
	}, nil
}

func (p *Post)loadAuthor() error {
	if author, err := (Author{}).GetOne(p.AuthorID); err != nil {
		return err
	} else {
		p.Author = *author
	}

	return nil
}

func (p *Post)loadMeta() error {

	var postMetas []PostMeta
	err := xormDb.Table("wprdh0703_postmeta").
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

	return nil
}

func (p *Post)loadCategoriesAndTerms() error {
	var terms []Term

	terms, err := (&Term{}).FindByPort(p.ID)

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


