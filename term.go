package main

import (
	"github.com/go-xorm/xorm"
	"strconv"
	"context"
	"errors"
)

type Term struct {
	ID int                `json:"id"  xorm:"term_id"`
	Taxonomy string				`json:"taxonomy"  xorm:"taxonomy"`//category or post_tag
	Name string           `json:"name"      xorm:"name"`
	Slug string           `json:"slug"      xorm:"slug"`
}
func (Term) TableName() (string) {
	return "wprdh0703_terms"
}

func (t *Term)getQueryBase(ctx context.Context) *xorm.Session {
	return GetDBConn(ctx).Table("wprdh0703_terms").Select("wprdh0703_terms.name, wprdh0703_terms.slug, wprdh0703_term_taxonomy.taxonomy").
		Join("INNER", "wprdh0703_term_taxonomy", "wprdh0703_terms.term_id = wprdh0703_term_taxonomy.term_id").
		Join("INNER", "wprdh0703_term_relationships", "wprdh0703_term_taxonomy.term_taxonomy_id = wprdh0703_term_relationships.term_taxonomy_id")
}

func (t *Term)FindByPost(ctx context.Context, postId int64) ([]Term, error) {
	var terms []Term

	err := t.getQueryBase(ctx).Where("wprdh0703_term_relationships.object_id = ?", postId).Find(&terms)

	if err != nil {
		return nil, err
	}

	return terms, nil
}

type TermCount struct {
	Term Term     `json:"term" xorm:"extends"`
	NumPosts int  `json:"numPosts" xorm:"cnt"`
}

func (Term)CountTerm(ctx context.Context) ([]TermCount, error) {
	query := `
		SELECT * FROM (
			SELECT
        e.term_id,
				e.name,
				e.slug,
				count(DISTINCT c.object_id) cnt
			FROM wprdh0703_term_relationships c
				JOIN wprdh0703_term_taxonomy d ON c.term_taxonomy_id = d.term_taxonomy_id
				JOIN wprdh0703_terms e ON d.term_id = e.term_id
				JOIN wprdh0703_posts f on f.ID = c.object_id and f.post_status = 'publish' and f.post_type = 'post'
			WHERE d.taxonomy = 'post_tag'
			GROUP BY e.term_id, e.name, e.slug
		) a WHERE CNT >= 2
    ORDER BY cnt DESC
		LIMIT 500
  `
	results, err := GetDBConn(ctx).QueryString(query)

	if err != nil {
		return nil, err
	}

	termCounts := make([]TermCount, 0)

	for _, eachResult := range results {
		termIdStr, _ := eachResult["term_id"]
		termId, _ := strconv.Atoi(termIdStr)
		name, _ := eachResult["name"]
		slug, _ := eachResult["slug"]
		count, _ := eachResult["count"]
		numPosts, _ := strconv.Atoi(count)

		term := Term{
			ID: termId,
			Name: name,
			Slug: slug,
			Taxonomy: "post_tag",
		}

		termCount := TermCount{
			Term: term,
			NumPosts: numPosts,
		}

		termCounts = append(termCounts, termCount)
	}

	return termCounts, nil
}

func (Term)FinyBySlug(ctx context.Context, slug string, taxonomy string) (*Term, error) {
	var term Term

	has, err := GetDBConn(ctx).Table("wprdh0703_terms").
		Select("wprdh0703_terms.term_id, wprdh0703_terms.name, wprdh0703_terms.slug, wprdh0703_term_taxonomy.taxonomy").
		Join("INNER", "wprdh0703_term_taxonomy", "wprdh0703_terms.term_id = wprdh0703_term_taxonomy.term_id and wprdh0703_term_taxonomy.taxonomy = ?", taxonomy).
		Where("wprdh0703_terms.slug = ?", slug).Get(&term)

	if !has {
		return nil, errors.New("No term [" + slug + "]")
	}

	if err != nil {
		return nil, err
	}

	return &term, nil
}