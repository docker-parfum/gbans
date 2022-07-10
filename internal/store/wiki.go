package store

import (
	"context"
	sq "github.com/Masterminds/squirrel"
	"github.com/leighmacdonald/gbans/pkg/wiki"
	log "github.com/sirupsen/logrus"
)

func (database *pgStore) GetWikiPageBySlug(ctx context.Context, slug string, page *wiki.Page) error {
	query, args, errQueryArgs := sb.Select("slug", "title", "body_md", "revision", "created_on", "updated_on").
		From("wiki").Where(sq.Eq{"slug": slug}).OrderBy("revision desc").Limit(1).ToSql()
	if errQueryArgs != nil {
		return Err(errQueryArgs)
	}
	if errQuery := database.conn.QueryRow(ctx, query, args...).Scan(&page.Slug, &page.Title, &page.BodyMD, &page.Revision,
		&page.CreatedOn, &page.UpdatedOn); errQuery != nil {
		return Err(errQuery)
	}
	return nil
}

func (database *pgStore) DeleteWikiPageBySlug(ctx context.Context, slug string) error {
	query, args, errQueryArgs := sb.Delete("wiki").Where(sq.Eq{"slug": slug}).ToSql()
	if errQueryArgs != nil {
		return errQueryArgs
	}
	if _, errExec := database.conn.Exec(ctx, query, args...); errExec != nil {
		return Err(errExec)
	}
	log.Debugf("Wiki slug deleted: %s", slug)
	return nil
}

func (database *pgStore) SaveWikiPage(ctx context.Context, page *wiki.Page) error {
	query, args, errQueryArgs := sb.Insert("wiki").
		Columns("slug", "title", "body_md", "revision", "created_on", "updated_on").
		Values(page.Slug, page.Title, page.BodyMD, page.Revision, page.CreatedOn, page.UpdatedOn).
		ToSql()
	if errQueryArgs != nil {
		return errQueryArgs
	}
	_, errQueryRow := database.conn.Exec(ctx, query, args...)
	if errQueryRow != nil {
		return Err(errQueryRow)
	}
	log.Debugf("Wiki page saved: %s", page.Title)
	return nil
}

func (database *pgStore) SaveWikiMedia(ctx context.Context, media *wiki.Media) error {
	const query = `
		INSERT INTO wiki_media (
		    author_id, mime_type, name, contents, size, deleted, created_on, updated_on
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING wiki_media_id
	`
	if errQuery := database.conn.QueryRow(ctx, query,
		media.AuthorId,
		media.MimeType,
		media.Name,
		media.Contents,
		media.Size,
		media.Deleted,
		media.CreatedOn,
		media.UpdatedOn,
	).Scan(&media.WikiMediaId); errQuery != nil {
		return Err(errQuery)
	}
	log.WithFields(log.Fields{
		"wiki_media_id": media.WikiMediaId,
		"author_id":     media.AuthorId,
		"name":          media.Name,
		"size":          media.Size,
		"mime":          media.MimeType,
	}).Infof("Wiki media created")
	return nil
}

func (database *pgStore) GetWikiMediaByName(ctx context.Context, name string, media *wiki.Media) error {
	const query = `
		SELECT 
		   wiki_media_id, author_id, name, size, mime_type, contents, deleted, created_on, updated_on
		FROM wiki_media
		WHERE deleted = false AND name = $1`
	if errQuery := database.conn.QueryRow(ctx, query, name).Scan(
		&media.WikiMediaId,
		&media.AuthorId,
		&media.Name,
		&media.Size,
		&media.MimeType,
		&media.Contents,
		&media.Deleted,
		&media.CreatedOn,
		&media.UpdatedOn,
	); errQuery != nil {
		return Err(errQuery)
	}
	return nil
}
