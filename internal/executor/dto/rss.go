package dto

import (
	"golang-stock-scryper/pkg/utils"
	"time"
)

type RSS struct {
	Channel RSSChannel `xml:"channel"`
}

type RSSChannel struct {
	Title string    `xml:"title"`
	Items []RSSItem `xml:"item"`
}

type RSSItem struct {
	Title   string     `xml:"title"`
	Link    string     `xml:"link"`
	PubDate *PubDate   `xml:"pubDate"`
	Source  *RSSSource `xml:"source"` // pakai pointer supaya bisa nil
}

type RSSSource struct {
	URL  string `xml:"url,attr"`
	Name string `xml:",chardata"`
}

type PubDate time.Time

func (p *PubDate) UnmarshalText(text []byte) error {
	layout := "Mon, 02 Jan 2006 15:04:05 MST"

	// Parse waktu dengan timezone awal (biasanya GMT)
	t, err := time.Parse(layout, string(text))
	if err != nil {
		return err
	}

	// Load lokasi Asia/Jakarta
	loc := utils.GetWibTimeLocation()

	// Konversi ke zona waktu Asia/Jakarta
	t = t.In(loc)

	*p = PubDate(t)
	return nil
}

func (p PubDate) Time() time.Time {
	return time.Time(p)
}

func (p PubDate) String() string {
	return time.Time(p).Format("Mon, 02 Jan 2006 15:04:05 MST")
}
