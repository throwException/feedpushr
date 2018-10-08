package filter

import (
	"sync/atomic"
	"time"

	"github.com/ncarlier/feedpushr/pkg/builder"

	"github.com/RadhiFadlillah/go-readability"
	"github.com/ncarlier/feedpushr/pkg/model"
)

// FetchFilter is a filter that try to fetch the original article content
type FetchFilter struct {
	name      string
	desc      string
	tags      []string
	nbError   uint64
	nbSuccess uint64
}

// DoFilter applies filter on the article
func (f *FetchFilter) DoFilter(article *model.Article) error {
	art, err := readability.Parse(article.Link, 5*time.Second)
	if err != nil {
		atomic.AddUint64(&f.nbError, 1)
		return err
	}
	article.Meta["RawContent"] = article.Content
	article.Content = art.Content
	article.Meta["MinReadTime"] = art.Meta.MinReadTime
	article.Meta["MaxReadTime"] = art.Meta.MaxReadTime
	atomic.AddUint64(&f.nbSuccess, 1)
	return nil
}

// GetSpec return filter specifications
func (f *FetchFilter) GetSpec() model.FilterSpec {
	result := model.FilterSpec{
		Name: f.name,
		Desc: f.desc,
		Tags: f.tags,
	}
	result.Props = map[string]interface{}{
		"nbError":    f.nbError,
		"nbSsuccess": f.nbSuccess,
	}
	return result
}

func newFetchFilter(tags string) *FetchFilter {
	return &FetchFilter{
		name: "fetch",
		desc: "This filter will attempt to extract the content of the article from the source URL.",
		tags: builder.GetFeedTags(&tags),
	}
}
