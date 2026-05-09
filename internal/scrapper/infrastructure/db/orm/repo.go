package orm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/orm/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/orm/query"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(ctx context.Context, dsn string) (*Repository, error) {
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("orm repo: open (%w)", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("orm repo: sql db (%w)", err)
	}
	if pingErr := sqlDB.PingContext(ctx); pingErr != nil {
		if closeErr := sqlDB.Close(); closeErr != nil {
			slog.Warn("orm repo: close after failed ping", slog.String("error", closeErr.Error()))
		}
		return nil, fmt.Errorf("orm repo: ping (%w)", pingErr)
	}
	return &Repository{db: gdb}, nil
}

func (r *Repository) Close() {
	sqlDB, err := r.db.DB()
	if err != nil {
		slog.Warn("orm repo: underlying sql db", slog.String("error", err.Error()))
		return
	}
	if closeErr := sqlDB.Close(); closeErr != nil {
		slog.Warn("orm repo: close", slog.String("error", closeErr.Error()))
	}
}

func (r *Repository) AddChat(ctx context.Context, id int64) error {
	c := model.Chat{TelegramID: id}
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&c)
	if res.Error != nil {
		return fmt.Errorf("orm repo: AddChat (%w)", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrChatAlreadyExists
	}
	return nil
}

func (r *Repository) DeleteChat(ctx context.Context, id int64) error {
	q := query.Use(r.db)
	info, err := q.Chat.WithContext(ctx).Where(q.Chat.TelegramID.Eq(id)).Delete()
	if err != nil {
		return fmt.Errorf("orm repo: DeleteChat (%w)", err)
	}
	if info.RowsAffected == 0 {
		return domain.ErrChatNotFound
	}
	return nil
}

func (r *Repository) AddLink(ctx context.Context, chatID int64, link string, tags, filters *[]string) error {
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return addLinkInTx(ctx, tx, chatID, link, tags, filters)
	}); err != nil {
		return fmt.Errorf("orm repo: AddLink: %w", err)
	}
	return nil
}

func (r *Repository) GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error) {
	q := query.Use(r.db)
	chat, err := q.Chat.WithContext(ctx).Where(q.Chat.TelegramID.Eq(chatID)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrChatNotFound
		}
		return nil, fmt.Errorf("orm repo: GetLinks load chat (%w)", err)
	}

	subs, err := q.Subscription.WithContext(ctx).
		Where(q.Subscription.ChatID.Eq(chat.ID)).
		Order(q.Subscription.ID).
		Find()
	if err != nil {
		return nil, fmt.Errorf("orm repo: GetLinks subscriptions (%w)", err)
	}
	if len(subs) == 0 {
		return nil, nil
	}
	out := make([]domain.Link, 0, len(subs))
	for _, sub := range subs {
		dl, bErr := r.domainLinkForSubscription(ctx, sub)
		if bErr != nil {
			return nil, bErr
		}
		out = append(out, dl)
	}
	return out, nil
}

func (r *Repository) ListSubscriptions(ctx context.Context, limit, offset int) ([]domain.Subscription, error) {
	q := query.Use(r.db)
	b := q.Subscription.WithContext(ctx).Order(q.Subscription.ID)
	if limit > 0 {
		b = b.Limit(limit).Offset(offset)
	}
	subs, err := b.Find()
	if err != nil {
		return nil, fmt.Errorf("orm repo: ListSubscriptions (%w)", err)
	}
	if len(subs) == 0 {
		return nil, nil
	}
	out := make([]domain.Subscription, 0, len(subs))
	for _, sub := range subs {
		chat, cErr := q.Chat.WithContext(ctx).Where(q.Chat.ID.Eq(sub.ChatID)).First()
		if cErr != nil {
			return nil, fmt.Errorf("orm repo: ListSubscriptions load chat (%w)", cErr)
		}
		dl, dErr := r.domainLinkForSubscription(ctx, sub)
		if dErr != nil {
			return nil, dErr
		}
		out = append(out, domain.Subscription{ChatID: chat.TelegramID, Link: dl})
	}
	return out, nil
}

func (r *Repository) DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error) {
	var out domain.Link
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		qu := query.Use(tx)
		chat, e := qu.Chat.WithContext(ctx).Where(qu.Chat.TelegramID.Eq(chatID)).First()
		if e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return domain.ErrChatNotFound
			}
			return fmt.Errorf("orm repo: DeleteLink load chat (%w)", e)
		}
		lnk, e := qu.Link.WithContext(ctx).Where(qu.Link.URL.Eq(linkURL)).First()
		if e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return domain.ErrLinkNotFound
			}
			return fmt.Errorf("orm repo: DeleteLink load link (%w)", e)
		}
		sub, e := qu.Subscription.WithContext(ctx).
			Where(qu.Subscription.ChatID.Eq(chat.ID)).
			Where(qu.Subscription.LinkID.Eq(lnk.ID)).
			First()
		if e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return domain.ErrLinkNotFound
			}
			return fmt.Errorf("orm repo: DeleteLink load subscription (%w)", e)
		}
		dl, e := r.domainLinkForSubscriptionInTx(ctx, tx, sub)
		if e != nil {
			return e
		}
		out = dl
		info, e := qu.Subscription.WithContext(ctx).Where(qu.Subscription.ID.Eq(sub.ID)).Delete()
		if e != nil {
			return fmt.Errorf("orm repo: DeleteLink delete (%w)", e)
		}
		if info.RowsAffected == 0 {
			return domain.ErrLinkNotFound
		}
		return nil
	})
	if err != nil {
		return domain.Link{}, fmt.Errorf("orm repo: DeleteLink: %w", err)
	}
	return out, nil
}

func (r *Repository) UpdateLinkUpdatedAt(ctx context.Context, chatID int64, linkURL string, t time.Time) error {
	q := query.Use(r.db)
	chat, err := q.Chat.WithContext(ctx).Where(q.Chat.TelegramID.Eq(chatID)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrLinkNotFound
		}
		return fmt.Errorf("orm repo: UpdateLinkUpdatedAt load chat (%w)", err)
	}
	lnk, err := q.Link.WithContext(ctx).Where(q.Link.URL.Eq(linkURL)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrLinkNotFound
		}
		return fmt.Errorf("orm repo: UpdateLinkUpdatedAt load link (%w)", err)
	}
	if _, err = q.Subscription.WithContext(ctx).
		Where(q.Subscription.ChatID.Eq(chat.ID)).
		Where(q.Subscription.LinkID.Eq(lnk.ID)).
		First(); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrLinkNotFound
		}
		return fmt.Errorf("orm repo: UpdateLinkUpdatedAt load subscription (%w)", err)
	}
	info, err := q.Link.WithContext(ctx).
		Where(q.Link.ID.Eq(lnk.ID)).
		Update(q.Link.LastUpdatedAt, t)
	if err != nil {
		return fmt.Errorf("orm repo: UpdateLinkUpdatedAt (%w)", err)
	}
	if info.RowsAffected == 0 {
		return domain.ErrLinkNotFound
	}
	return nil
}

func (r *Repository) CreateTag(ctx context.Context, value string) (int64, error) {
	if value == "" {
		return 0, errors.New("orm repo: CreateTag - empty value")
	}
	q := query.Use(r.db)
	row := &model.Tag{Value: value}
	if err := q.Tag.WithContext(ctx).Create(row); err != nil {
		if isPGUniqueViolation(err) {
			return 0, domain.ErrTagAlreadyExists
		}
		return 0, fmt.Errorf("orm repo: CreateTag (%w)", err)
	}
	return row.ID, nil
}

func (r *Repository) ListTags(ctx context.Context, limit, offset int) ([]domain.Tag, error) {
	q := query.Use(r.db)
	b := q.Tag.WithContext(ctx).Order(q.Tag.ID)
	if limit > 0 {
		b = b.Limit(limit).Offset(offset)
	}
	items, err := b.Find()
	if err != nil {
		return nil, fmt.Errorf("orm repo: ListTags (%w)", err)
	}
	out := make([]domain.Tag, 0, len(items))
	for _, it := range items {
		out = append(out, domain.Tag{ID: it.ID, Value: it.Value})
	}
	return out, nil
}

func (r *Repository) UpdateTag(ctx context.Context, id int64, value string) error {
	if value == "" {
		return errors.New("orm repo: UpdateTag - empty value")
	}
	q := query.Use(r.db)
	info, err := q.Tag.WithContext(ctx).Where(q.Tag.ID.Eq(id)).Update(q.Tag.Value, value)
	if err != nil {
		if isPGUniqueViolation(err) {
			return domain.ErrTagAlreadyExists
		}
		return fmt.Errorf("orm repo: UpdateTag (%w)", err)
	}
	if info.RowsAffected == 0 {
		return domain.ErrTagNotFound
	}
	return nil
}

func (r *Repository) DeleteTag(ctx context.Context, id int64) error {
	q := query.Use(r.db)
	info, err := q.Tag.WithContext(ctx).Where(q.Tag.ID.Eq(id)).Delete()
	if err != nil {
		return fmt.Errorf("orm repo: DeleteTag (%w)", err)
	}
	if info.RowsAffected == 0 {
		return domain.ErrTagNotFound
	}
	return nil
}

func (r *Repository) domainLinkForSubscription(ctx context.Context, sub *model.Subscription) (domain.Link, error) {
	q := query.Use(r.db)
	lnk, err := q.Link.WithContext(ctx).Where(q.Link.ID.Eq(sub.LinkID)).First()
	if err != nil {
		return domain.Link{}, fmt.Errorf("orm repo: load link (%w)", err)
	}
	tags, err := r.tagValuesForSubscription(ctx, sub.ID)
	if err != nil {
		return domain.Link{}, err
	}
	filters, err := r.filterValuesForSubscription(ctx, sub.ID)
	if err != nil {
		return domain.Link{}, err
	}
	dl := domain.Link{URL: lnk.URL, Tags: tags, Filters: filters}
	if !lnk.LastUpdatedAt.IsZero() {
		dl.LastUpdated = lnk.LastUpdatedAt
	}
	return dl, nil
}

func (r *Repository) tagValuesForSubscription(ctx context.Context, subID int64) ([]string, error) {
	q := query.Use(r.db)
	rows, err := q.Tag.WithContext(ctx).
		Select(q.Tag.Value).
		Join(q.LinkTag, q.LinkTag.TagID.EqCol(q.Tag.ID)).
		Where(q.LinkTag.SubscriptionID.Eq(subID)).
		Order(q.Tag.Value).
		Find()
	if err != nil {
		return nil, fmt.Errorf("orm repo: load tags for subscription: %w", err)
	}
	out := make([]string, 0, len(rows))
	for _, t := range rows {
		out = append(out, t.Value)
	}
	return out, nil
}

func (r *Repository) filterValuesForSubscription(ctx context.Context, subID int64) ([]string, error) {
	q := query.Use(r.db)
	rows, err := q.Filter.WithContext(ctx).
		Select(q.Filter.Value).
		Join(q.LinkFilter, q.LinkFilter.FilterID.EqCol(q.Filter.ID)).
		Where(q.LinkFilter.SubscriptionID.Eq(subID)).
		Order(q.Filter.Value).
		Find()
	if err != nil {
		return nil, fmt.Errorf("orm repo: load filters for subscription: %w", err)
	}
	out := make([]string, 0, len(rows))
	for _, f := range rows {
		out = append(out, f.Value)
	}
	return out, nil
}

func (r *Repository) domainLinkForSubscriptionInTx(ctx context.Context, tx *gorm.DB, sub *model.Subscription) (domain.Link, error) {
	q := query.Use(tx)
	lnk, err := q.Link.WithContext(ctx).Where(q.Link.ID.Eq(sub.LinkID)).First()
	if err != nil {
		return domain.Link{}, fmt.Errorf("orm repo: load link (%w)", err)
	}
	tags, err := r.tagValuesForSubscriptionTx(ctx, tx, sub.ID)
	if err != nil {
		return domain.Link{}, err
	}
	filters, err := r.filterValuesForSubscriptionTx(ctx, tx, sub.ID)
	if err != nil {
		return domain.Link{}, err
	}
	dl := domain.Link{URL: lnk.URL, Tags: tags, Filters: filters}
	if !lnk.LastUpdatedAt.IsZero() {
		dl.LastUpdated = lnk.LastUpdatedAt
	}
	return dl, nil
}

func (r *Repository) tagValuesForSubscriptionTx(ctx context.Context, tx *gorm.DB, subID int64) ([]string, error) {
	q := query.Use(tx)
	rows, err := q.Tag.WithContext(ctx).
		Select(q.Tag.Value).
		Join(q.LinkTag, q.LinkTag.TagID.EqCol(q.Tag.ID)).
		Where(q.LinkTag.SubscriptionID.Eq(subID)).
		Order(q.Tag.Value).
		Find()
	if err != nil {
		return nil, fmt.Errorf("orm repo: load tags for subscription: %w", err)
	}
	out := make([]string, 0, len(rows))
	for _, t := range rows {
		out = append(out, t.Value)
	}
	return out, nil
}

func (r *Repository) filterValuesForSubscriptionTx(ctx context.Context, tx *gorm.DB, subID int64) ([]string, error) {
	q := query.Use(tx)
	rows, err := q.Filter.WithContext(ctx).
		Select(q.Filter.Value).
		Join(q.LinkFilter, q.LinkFilter.FilterID.EqCol(q.Filter.ID)).
		Where(q.LinkFilter.SubscriptionID.Eq(subID)).
		Order(q.Filter.Value).
		Find()
	if err != nil {
		return nil, fmt.Errorf("orm repo: load filters for subscription: %w", err)
	}
	out := make([]string, 0, len(rows))
	for _, f := range rows {
		out = append(out, f.Value)
	}
	return out, nil
}

func addLinkInTx(ctx context.Context, tx *gorm.DB, chatID int64, link string, tags, filters *[]string) error {
	qu := query.Use(tx)
	chat, err := qu.Chat.WithContext(ctx).Where(qu.Chat.TelegramID.Eq(chatID)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrChatNotFound
		}
		return fmt.Errorf("orm repo: AddLink load chat (%w)", err)
	}

	linkRow := &model.Link{URL: link}
	if linkErr := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "url"}},
		DoUpdates: clause.AssignmentColumns([]string{"url"}),
	}).Create(linkRow).Error; linkErr != nil {
		return fmt.Errorf("orm repo: AddLink link upsert (%w)", linkErr)
	}

	sub := &model.Subscription{ChatID: chat.ID, LinkID: linkRow.ID}
	res := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "link_id"}},
		DoNothing: true,
	}).Create(sub)
	if res.Error != nil {
		return fmt.Errorf("orm repo: AddLink subscription (%w)", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrLinkAlreadyExists
	}

	tagVals := []string{}
	if tags != nil {
		tagVals = *tags
	}
	for _, v := range tagVals {
		if v == "" {
			continue
		}
		tagID, tagErr := getOrCreateTagID(ctx, tx, v)
		if tagErr != nil {
			return tagErr
		}
		lt := &model.LinkTag{SubscriptionID: sub.ID, TagID: tagID}
		if cErr := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "subscription_id"}, {Name: "tag_id"}},
			DoNothing: true,
		}).Create(lt).Error; cErr != nil {
			return fmt.Errorf("orm repo: AddLink link_tag (%w)", cErr)
		}
	}

	filterVals := []string{}
	if filters != nil {
		filterVals = *filters
	}
	for _, v := range filterVals {
		if v == "" {
			continue
		}
		filterID, filterErr := getOrCreateFilterID(ctx, tx, v)
		if filterErr != nil {
			return filterErr
		}
		lf := &model.LinkFilter{SubscriptionID: sub.ID, FilterID: filterID}
		if cErr := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "subscription_id"}, {Name: "filter_id"}},
			DoNothing: true,
		}).Create(lf).Error; cErr != nil {
			return fmt.Errorf("orm repo: AddLink link_filter (%w)", cErr)
		}
	}
	return nil
}

func getOrCreateTagID(ctx context.Context, tx *gorm.DB, value string) (int64, error) {
	t := &model.Tag{Value: value}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "value"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(t).Error; err != nil {
		return 0, fmt.Errorf("orm repo: tag upsert (%w)", err)
	}
	return t.ID, nil
}

func getOrCreateFilterID(ctx context.Context, tx *gorm.DB, value string) (int64, error) {
	f := &model.Filter{Value: value}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "value"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(f).Error; err != nil {
		return 0, fmt.Errorf("orm repo: filter upsert (%w)", err)
	}
	return f.ID, nil
}
