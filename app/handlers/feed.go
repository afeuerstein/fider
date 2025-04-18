package handlers

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"time"

	"github.com/getfider/fider/app/models/query"
	"github.com/getfider/fider/app/pkg/bus"
	"github.com/getfider/fider/app/pkg/web"
)

type AtomFeed struct {
	XMLName  xml.Name `xml:"http://www.w3.org/2005/Atom feed"`
	Title    string   `xml:"title"`
	Subtitle Content  `xml:"subtitle"`
	Id       string   `xml:"id"`
	Updated  string   `xml:"updated"`
	Link     []Link   `xml:"link"`
	Author   *Author  `xml:"author"`
	Entries  []*Entry `xml:"entry"`
}

type Entry struct {
	Title     string   `xml:"title"`
	Id        string   `xml:"id"`
	Published string   `xml:"published"`
	Updated   string   `xml:"updated,omitempty"`
	Link      []Link   `xml:"link"`
	Author    *Author  `xml:"author"`
	Summary   *Content `xml:"summary"`
	Content   *Content `xml:"content"`
}

type Link struct {
	Rel      string `xml:"rel,attr,omitempty"`
	Href     string `xml:"href,attr"`
	Type     string `xml:"type,attr,omitempty"`
	HrefLang string `xml:"hreflang,attr,omitempty"`
	Title    string `xml:"title,attr,omitempty"`
	Length   uint   `xml:"length,attr,omitempty"`
}

type Author struct {
	Name     string `xml:"name"`
	Uri      string `xml:"uri,omitempty"`
	Email    string `xml:"email,omitempty"`
	InnerXML string `xml:",innerxml"`
}

type Content struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

func formatTime(t time.Time) string {
	return t.Format("2006-01-02T15:04:05-07:00")
}

// Returns the global ATOM feed with the 30 most recent posts as entries
func GlobalFeed() web.HandlerFunc {
	return func(c *web.Context) error {
		searchPosts := &query.SearchPosts{
			Query: c.QueryParam("query"),
			View:  "all",
			Limit: "30", //c.QueryParam("limit"),
			Tags:  c.QueryParamAsArray("tags"),
		}
		if err := bus.Dispatch(c, searchPosts); err != nil {
			return c.Failure(err)
		}
		posts := searchPosts.Result

		feed := &AtomFeed{
			Title:    c.Tenant().Name,
			Subtitle: Content{Body: c.Tenant().WelcomeMessage}, // not implemented by atom package
			Id:       web.BaseURL(c),
			Link: []Link{
				{Href: fmt.Sprintf("%s/feed.atom", web.BaseURL(c)), Type: "application/atom+xml", Rel: "self"},
				{Href: web.BaseURL(c), Type: "text/html", Rel: "alternate"},
			},
		}

		feed.Entries = []*Entry{}
		lastUpdate := time.UnixMilli(0)
		for _, post := range posts {
			if post.CreatedAt.After(lastUpdate) {
				lastUpdate = post.CreatedAt
			}
			if (post.Response != nil) && post.Response.RespondedAt.After(lastUpdate) {
				lastUpdate = post.Response.RespondedAt
			}

			feed.Entries = append(feed.Entries, &Entry{
				Title:     post.Title,
				Author:    &Author{Name: post.User.Name},
				Published: formatTime(post.CreatedAt),
				Updated: func() string {
					if post.Response == nil {
						return ""
					}
					return formatTime(post.Response.RespondedAt)
				}(),
				Summary: &Content{Type: "html", Body: post.Description},
				Id:      fmt.Sprintf("%s/posts/%d", web.BaseURL(c), post.ID),
				Link: []Link{
					{Href: fmt.Sprintf("%s/feed/posts/%d.atom", web.BaseURL(c), post.ID), Type: "application/atom+xml", Rel: "self"},
					{Href: fmt.Sprintf("%s/posts/%d", web.BaseURL(c), post.ID), Type: "text/html", Rel: "alternate"},
				},
			})
		}
		feed.Updated = formatTime(lastUpdate)

		feedXML, err := xml.MarshalIndent(feed, "", "	")
		if err != nil {
			return c.Failure(err)
		}
		feedStr := "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n" + string(feedXML)

		return c.Blob(http.StatusOK, "application/atom+xml", []byte(feedStr))
	}
}
