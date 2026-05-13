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

func addLinkInTx(ctx context.Context, tx *gorm.DB, chatID int64, link string, tags, filters *[]string) error {
	internalChatID, err := resolveChatPK(ctx, tx, chatID)
	if err != nil {
		return err
	}
	linkRow, err := getOrInsertLink(ctx, tx, link)
	if err != nil {
		return err
	}
	subRow, err := insertSubscription(ctx, tx, internalChatID, linkRow.ID)
	if err != nil {
		return err
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
		row := model.LinkTag{SubscriptionID: subRow.ID, TagID: tagID}
		if juncErr := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; juncErr != nil {
			return fmt.Errorf("orm repo: AddLink link_tag (%w)", juncErr)
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
		row := model.LinkFilter{SubscriptionID: subRow.ID, FilterID: filterID}
		if juncErr := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; juncErr != nil {
			return fmt.Errorf("orm repo: AddLink link_filter (%w)", juncErr)
		}
	}
	return nil
}

func resolveChatPK(ctx context.Context, tx *gorm.DB, telegramID int64) (int64, error) {
	var chatRow model.Chat
	if err := tx.WithContext(ctx).Where("telegram_id = ?", telegramID).Take(&chatRow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, domain.ErrChatNotFound
		}
		return 0, fmt.Errorf("orm repo: AddLink load chat (%w)", err)
	}
	return chatRow.ID, nil
}

func getOrInsertLink(ctx context.Context, tx *gorm.DB, url string) (*model.Link, error) {
	var linkRow model.Link
	switch err := tx.WithContext(ctx).Where("url = ?", url).Take(&linkRow).Error; {
	case err == nil:
		return &linkRow, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		linkRow = model.Link{URL: url}
		if creErr := tx.WithContext(ctx).Create(&linkRow).Error; creErr != nil {
			if !isPGUniqueViolation(creErr) {
				return nil, fmt.Errorf("orm repo: AddLink create link (%w)", creErr)
			}
			if takeErr := tx.WithContext(ctx).Where("url = ?", url).Take(&linkRow).Error; takeErr != nil {
				return nil, fmt.Errorf("orm repo: AddLink load link after conflict (%w)", takeErr)
			}
		}
		return &linkRow, nil
	default:
		return nil, fmt.Errorf("orm repo: AddLink load link (%w)", err)
	}
}

func insertSubscription(ctx context.Context, tx *gorm.DB, chatPK, linkID int64) (*model.Subscription, error) {
	var existing model.Subscription
	switch err := tx.WithContext(ctx).Where("chat_id = ? AND link_id = ?", chatPK, linkID).Take(&existing).Error; {
	case err == nil:
		return nil, domain.ErrLinkAlreadyExists
	case errors.Is(err, gorm.ErrRecordNotFound):
	default:
		return nil, fmt.Errorf("orm repo: AddLink load subscription (%w)", err)
	}
	sub := model.Subscription{ChatID: chatPK, LinkID: linkID}
	if err := tx.WithContext(ctx).Create(&sub).Error; err != nil {
		if isPGUniqueViolation(err) {
			return nil, domain.ErrLinkAlreadyExists
		}
		return nil, fmt.Errorf("orm repo: AddLink create subscription (%w)", err)
	}
	return &sub, nil
}

func getOrCreateTagID(ctx context.Context, tx *gorm.DB, value string) (int64, error) {
	var t model.Tag
	if err := tx.WithContext(ctx).Where(model.Tag{Value: value}).FirstOrCreate(&t).Error; err != nil {
		return 0, fmt.Errorf("orm repo: tag (%w)", err)
	}
	return t.ID, nil
}

func getOrCreateFilterID(ctx context.Context, tx *gorm.DB, value string) (int64, error) {
	var f model.Filter
	if err := tx.WithContext(ctx).Where(model.Filter{Value: value}).FirstOrCreate(&f).Error; err != nil {
		return 0, fmt.Errorf("orm repo: filter (%w)", err)
	}
	return f.ID, nil
}

type subJoinValue struct {
	SubscriptionID int64  `gorm:"column:subscription_id"`
	Value          string `gorm:"column:value"`
}
type linkStamp struct {
	ID            int64     `gorm:"column:id"`
	URL           string    `gorm:"column:url"`
	LastUpdatedAt time.Time `gorm:"column:last_updated_at"`
}

func subscriptionTagValues(ctx context.Context, db *gorm.DB, subIDs []int64) (map[int64][]string, error) {
	if len(subIDs) == 0 {
		return map[int64][]string{}, nil
	}
	var rows []subJoinValue
	err := db.WithContext(ctx).Model(&model.LinkTag{}).
		Select("link_tag.subscription_id, tag.value").
		Joins("INNER JOIN tag ON tag.id = link_tag.tag_id").
		Where("link_tag.subscription_id IN ?", subIDs).
		Order("link_tag.subscription_id, tag.value").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("orm repo: subscription tags (%w)", err)
	}
	out := make(map[int64][]string)
	for _, row := range rows {
		out[row.SubscriptionID] = append(out[row.SubscriptionID], row.Value)
	}
	return out, nil
}

func subscriptionFilterValues(ctx context.Context, db *gorm.DB, subIDs []int64) (map[int64][]string, error) {
	if len(subIDs) == 0 {
		return map[int64][]string{}, nil
	}
	var rows []subJoinValue
	err := db.WithContext(ctx).Model(&model.LinkFilter{}).
		Select("link_filter.subscription_id, filter.value").
		Joins("INNER JOIN filter ON filter.id = link_filter.filter_id").
		Where("link_filter.subscription_id IN ?", subIDs).
		Order("link_filter.subscription_id, filter.value").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("orm repo: subscription filters (%w)", err)
	}
	out := make(map[int64][]string)
	for _, row := range rows {
		out[row.SubscriptionID] = append(out[row.SubscriptionID], row.Value)
	}
	return out, nil
}

func linkByIDFromSubscriptions(ctx context.Context, db *gorm.DB, subs []model.Subscription) (map[int64]linkStamp, error) {
	seen := make(map[int64]struct{})
	var ids []int64
	for _, s := range subs {
		if _, ok := seen[s.LinkID]; !ok {
			seen[s.LinkID] = struct{}{}
			ids = append(ids, s.LinkID)
		}
	}
	if len(ids) == 0 {
		return map[int64]linkStamp{}, nil
	}
	var stamps []linkStamp
	if err := db.WithContext(ctx).Table(model.TableNameLink).Where("id IN ?", ids).Find(&stamps).Error; err != nil {
		return nil, fmt.Errorf("orm repo: load links by id (%w)", err)
	}
	out := make(map[int64]linkStamp, len(stamps))
	for _, lk := range stamps {
		out[lk.ID] = lk
	}
	return out, nil
}

func linksFromSubscriptions(ctx context.Context, db *gorm.DB, subs []model.Subscription) ([]domain.Link, error) {
	if len(subs) == 0 {
		return nil, nil
	}
	subIDs := make([]int64, len(subs))
	for i, s := range subs {
		subIDs[i] = s.ID
	}
	linkByID, err := linkByIDFromSubscriptions(ctx, db, subs)
	if err != nil {
		return nil, err
	}
	tagsBySub, err := subscriptionTagValues(ctx, db, subIDs)
	if err != nil {
		return nil, err
	}
	filtersBySub, err := subscriptionFilterValues(ctx, db, subIDs)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Link, 0, len(subs))
	for _, s := range subs {
		lk := linkByID[s.LinkID]
		tags := tagsBySub[s.ID]
		if tags == nil {
			tags = []string{}
		}
		filters := filtersBySub[s.ID]
		if filters == nil {
			filters = []string{}
		}
		dl := domain.Link{URL: lk.URL, Tags: tags, Filters: filters}
		if !lk.LastUpdatedAt.IsZero() {
			dl.LastUpdated = lk.LastUpdatedAt
		}
		out = append(out, dl)
	}
	return out, nil
}

func (r *Repository) GetLinks(ctx context.Context, chatID int64, limit, offset int) ([]domain.Link, error) {
	db := r.db.WithContext(ctx)
	var chat model.Chat
	if err := db.Where("telegram_id = ?", chatID).Take(&chat).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrChatNotFound
		}
		return nil, fmt.Errorf("orm repo: GetLinks chat (%w)", err)
	}
	q := db.Where("chat_id = ?", chat.ID).Order("id")
	if limit > 0 {
		q = q.Limit(limit).Offset(offset)
	}
	var subs []model.Subscription
	if err := q.Find(&subs).Error; err != nil {
		return nil, fmt.Errorf("orm repo: GetLinks subscriptions (%w)", err)
	}
	if len(subs) == 0 {
		return []domain.Link{}, nil
	}
	out, err := linksFromSubscriptions(ctx, db, subs)
	if err != nil {
		return nil, fmt.Errorf("orm repo: GetLinks (%w)", err)
	}
	return out, nil
}

type listSubscribedRow struct {
	SubID       int64     `gorm:"column:sub_id"`
	TelegramID  int64     `gorm:"column:telegram_id"`
	LinkURL     string    `gorm:"column:url"`
	LastUpdated time.Time `gorm:"column:last_updated_at"`
}

func (r *Repository) ListSubscribedLinks(ctx context.Context, limit, offset int) ([]domain.SubscribedLink, error) {
	db := r.db.WithContext(ctx)
	q := db.Table("subscriptions AS s").
		Select("s.id AS sub_id, c.telegram_id, l.url, l.last_updated_at AS last_updated_at").
		Joins("INNER JOIN chats c ON c.id = s.chat_id").
		Joins("INNER JOIN links l ON l.id = s.link_id").
		Order("c.telegram_id, s.id")
	if limit > 0 {
		q = q.Limit(limit).Offset(offset)
	}
	var rows []listSubscribedRow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("orm repo: ListSubscribedLinks (%w)", err)
	}
	if len(rows) == 0 {
		return []domain.SubscribedLink{}, nil
	}
	subIDs := make([]int64, len(rows))
	for i := range rows {
		subIDs[i] = rows[i].SubID
	}
	tagsBySub, err := subscriptionTagValues(ctx, db, subIDs)
	if err != nil {
		return nil, err
	}
	filtersBySub, err := subscriptionFilterValues(ctx, db, subIDs)
	if err != nil {
		return nil, err
	}
	out := make([]domain.SubscribedLink, 0, len(rows))
	for _, row := range rows {
		tags := tagsBySub[row.SubID]
		if tags == nil {
			tags = []string{}
		}
		filters := filtersBySub[row.SubID]
		if filters == nil {
			filters = []string{}
		}
		l := domain.Link{URL: row.LinkURL, Tags: tags, Filters: filters}
		if !row.LastUpdated.IsZero() {
			l.LastUpdated = row.LastUpdated
		}
		out = append(out, domain.SubscribedLink{ChatID: row.TelegramID, Link: l})
	}
	return out, nil
}

// DeleteLink удаляет подписку и возвращает ссылку для ответа API.
// Сначала читаются теги/фильтры и last_updated_at ссылки: после DELETE по подписке
// каскадом удалятся link_tag/link_filter, без предварительного чтения их не вернуть.
func (r *Repository) DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error) {
	var out domain.Link
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var chat model.Chat
		if err := tx.Where("telegram_id = ?", chatID).Take(&chat).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrChatNotFound
			}
			return fmt.Errorf("orm repo: DeleteLink chat (%w)", err)
		}
		var link model.Link
		if err := tx.Where("url = ?", linkURL).Take(&link).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrLinkNotFound
			}
			return fmt.Errorf("orm repo: DeleteLink link (%w)", err)
		}
		var sub model.Subscription
		if err := tx.Where("chat_id = ? AND link_id = ?", chat.ID, link.ID).Take(&sub).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrLinkNotFound
			}
			return fmt.Errorf("orm repo: DeleteLink subscription (%w)", err)
		}
		tagsBySub, err := subscriptionTagValues(ctx, tx, []int64{sub.ID})
		if err != nil {
			return err
		}
		filtersBySub, err := subscriptionFilterValues(ctx, tx, []int64{sub.ID})
		if err != nil {
			return err
		}
		tags := tagsBySub[sub.ID]
		if tags == nil {
			tags = []string{}
		}
		filters := filtersBySub[sub.ID]
		if filters == nil {
			filters = []string{}
		}
		var stamp linkStamp
		if stampErr := tx.Table(model.TableNameLink).Where("id = ?", link.ID).Take(&stamp).Error; stampErr != nil {
			return fmt.Errorf("orm repo: DeleteLink link stamp (%w)", stampErr)
		}
		out = domain.Link{URL: stamp.URL, Tags: tags, Filters: filters}
		if !stamp.LastUpdatedAt.IsZero() {
			out.LastUpdated = stamp.LastUpdatedAt
		}
		if delErr := tx.Delete(&model.Subscription{}, sub.ID).Error; delErr != nil {
			return fmt.Errorf("orm repo: DeleteLink delete (%w)", delErr)
		}
		return nil
	})
	if err != nil {
		return domain.Link{}, fmt.Errorf("orm repo: DeleteLink transaction: %w", err)
	}
	return out, nil
}

func (r *Repository) UpdateLinkUpdatedAt(ctx context.Context, chatID int64, linkURL string, t time.Time) error {
	db := r.db.WithContext(ctx)
	var chat model.Chat
	if err := db.Where("telegram_id = ?", chatID).Take(&chat).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrChatNotFound
		}
		return fmt.Errorf("orm repo: UpdateLinkUpdatedAt chat (%w)", err)
	}
	var link model.Link
	if err := db.Where("url = ?", linkURL).Take(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrLinkNotFound
		}
		return fmt.Errorf("orm repo: UpdateLinkUpdatedAt link (%w)", err)
	}
	res := db.Model(&model.Link{}).
		Where("id = ?", link.ID).
		Update("last_updated_at", t)
	if res.Error != nil {
		return fmt.Errorf("orm repo: UpdateLinkUpdatedAt (%w)", res.Error)
	}
	if res.RowsAffected == 0 {
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
