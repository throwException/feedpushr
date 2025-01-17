package controller

import (
	"fmt"
	"time"

	"github.com/goadesign/goa"
	"github.com/ncarlier/feedpushr/v3/autogen/app"
	"github.com/ncarlier/feedpushr/v3/pkg/aggregator"
	"github.com/ncarlier/feedpushr/v3/pkg/common"
	"github.com/ncarlier/feedpushr/v3/pkg/feed"
	"github.com/ncarlier/feedpushr/v3/pkg/helper"
	"github.com/ncarlier/feedpushr/v3/pkg/model"
	"github.com/ncarlier/feedpushr/v3/pkg/store"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// FeedController implements the feed resource.
type FeedController struct {
	*goa.Controller
	db         store.DB
	aggregator *aggregator.Manager
	log        zerolog.Logger
}

// NewFeedController creates a feed controller.
func NewFeedController(service *goa.Service, db store.DB, am *aggregator.Manager) *FeedController {
	return &FeedController{
		Controller: service.NewController("FeedController"),
		db:         db,
		aggregator: am,
		log:        log.With().Str("component", "feed-ctrl").Logger(),
	}
}

// Create creates a new feed
func (c *FeedController) Create(ctx *app.CreateFeedContext) error {
	_feed, err := feed.NewFeed(ctx.URL, ctx.Tags)
	if err != nil {
		return ctx.BadRequest(goa.ErrBadRequest(err))
	}
	if !helper.IsEmptyString(ctx.Title) {
		_feed.Title = *ctx.Title
	}
	status := aggregator.StoppedStatus.String()
	if ctx.Enable != nil && *ctx.Enable {
		status = aggregator.RunningStatus.String()
	}
	_feed.Status = &status
	err = c.db.SaveFeed(_feed)
	if err != nil {
		return goa.ErrInternal(err)
	}
	if _feed.Status != nil && *_feed.Status == aggregator.RunningStatus.String() {
		fa := c.aggregator.RegisterFeedAggregator(_feed, 0)
		fa.Start()
		c.log.Info().Str("id", _feed.ID).Msg("feed created and registered")
	} else {
		c.log.Info().Str("id", _feed.ID).Msg("feed created")
	}

	return ctx.Created(feed.NewFeedResponseFromDef(_feed))
}

// Update updates a new feed
func (c *FeedController) Update(ctx *app.UpdateFeedContext) error {
	// Get feed from the database
	_feed, err := c.db.GetFeed(ctx.ID)
	if err != nil {
		if err == common.ErrFeedNotFound {
			return ctx.NotFound()
		}
		return goa.ErrInternal(err)
	}

	if ctx.Tags == nil && helper.IsEmptyString(ctx.Title) {
		return ctx.OK(feed.NewFeedResponseFromDef(_feed))
	}

	// Update feed title
	if !helper.IsEmptyString(ctx.Title) {
		_feed.Title = *ctx.Title
	}
	// Update feed tags
	_feed.Tags = feed.GetFeedTags(ctx.Tags)
	_feed.Mdate = time.Now()
	err = c.db.SaveFeed(_feed)
	if err != nil {
		return goa.ErrInternal(err)
	}
	fa := c.aggregator.GetFeedAggregator(_feed.ID)
	if fa != nil {
		// Reload aggregator data
		// For now we are recreating the aggregator
		c.aggregator.UnRegisterFeedAggregator(_feed.ID)
		if _feed.Status != nil && *_feed.Status == aggregator.RunningStatus.String() {
			c.aggregator.RegisterFeedAggregator(_feed, 0)
			c.log.Info().Str("id", _feed.ID).Msg("feed updated and aggregation restarted")
		} else {
			c.log.Info().Str("id", _feed.ID).Msg("feed updated and aggregation stopped")
		}
	} else {
		c.log.Info().Str("id", _feed.ID).Msg("feed updated")
	}

	return ctx.OK(feed.NewFeedResponseFromDef(_feed))
}

// Delete removes a feed
func (c *FeedController) Delete(ctx *app.DeleteFeedContext) error {
	c.aggregator.UnRegisterFeedAggregator(ctx.ID)
	_, err := c.db.DeleteFeed(ctx.ID)
	if err != nil {
		if err == common.ErrFeedNotFound {
			return ctx.NotFound()
		}
		return goa.ErrInternal(err)
	}
	c.log.Info().Str("id", ctx.ID).Msg("feed removed and aggregation stopped")
	return ctx.NoContent()
}

// List shows all feeds
func (c *FeedController) List(ctx *app.ListFeedContext) error {
	var page *model.FeedDefPage
	var err error
	if ctx.Q != nil && *ctx.Q != "" {
		page, err = c.db.SearchFeeds(*ctx.Q, ctx.Page, ctx.Size)
	} else {
		page, err = c.db.ListFeeds(ctx.Page, ctx.Size)
	}
	if err != nil {
		return goa.ErrInternal(err)
	}

	data := app.FeedResponseCollection{}
	for i := 0; i < len(page.Feeds); i++ {
		_feed := page.Feeds[i]
		// Get feed details with aggregation status
		fa := c.aggregator.GetFeedAggregator(_feed.ID)
		if fa != nil {
			data = append(data, fa.GetFeedWithAggregationStatus())
		} else {
			data = append(data, feed.NewFeedResponseFromDef(&_feed))
		}
	}
	res := &app.FeedsPageResponse{
		Size:    page.Size,
		Current: page.Page,
		Total:   page.Total,
		Data:    data,
	}
	return ctx.OK(res)
}

// Get shows a feed
func (c *FeedController) Get(ctx *app.GetFeedContext) error {
	// Get feed from the database
	_feed, err := c.db.GetFeed(ctx.ID)
	if err != nil {
		if err == common.ErrFeedNotFound {
			return ctx.NotFound()
		}
		return goa.ErrInternal(err)
	}
	// Get feed details with aggregation status
	fa := c.aggregator.GetFeedAggregator(_feed.ID)
	if fa != nil {
		return ctx.OK(fa.GetFeedWithAggregationStatus())
	}
	return ctx.OK(feed.NewFeedResponseFromDef(_feed))
}

// Start starts feed aggregation
func (c *FeedController) Start(ctx *app.StartFeedContext) error {
	_feed, err := c.db.GetFeed(ctx.ID)
	if err != nil {
		if err == common.ErrFeedNotFound {
			return ctx.NotFound()
		}
		return goa.ErrInternal(err)
	}
	// Start feed aggregation
	fa := c.aggregator.GetFeedAggregator(_feed.ID)
	if fa == nil {
		fa = c.aggregator.RegisterFeedAggregator(_feed, 0)
	}
	fa.StartWithDelay(0)
	// Update feed DB status
	status := aggregator.RunningStatus.String()
	_feed.Status = &status
	err = c.db.SaveFeed(_feed)
	if err != nil {
		return goa.ErrInternal(err)
	}
	c.log.Info().Str("id", _feed.ID).Msg("feed aggregation started")
	return ctx.Accepted()
}

// Stop stops feed aggregation
func (c *FeedController) Stop(ctx *app.StopFeedContext) error {
	_feed, err := c.db.GetFeed(ctx.ID)
	if err != nil {
		if err == common.ErrFeedNotFound {
			return ctx.NotFound()
		}
		return goa.ErrInternal(err)
	}
	// Stop feed aggregation
	fa := c.aggregator.GetFeedAggregator(_feed.ID)
	if fa == nil {
		return goa.ErrInternal(fmt.Errorf("feed aggregator not registered"))
	}
	fa.Stop()
	// Update feed DB status
	status := aggregator.StoppedStatus.String()
	_feed.Status = &status
	err = c.db.SaveFeed(_feed)
	if err != nil {
		return goa.ErrInternal(err)
	}
	c.log.Info().Str("id", _feed.ID).Msg("feed aggregation stopped")
	return ctx.Accepted()
}
