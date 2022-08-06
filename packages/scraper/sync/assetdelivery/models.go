package assetdelivery

import (
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
)

//easyjson:json
type Error struct {
	Code            int    `json:"code"`
	Message         string `json:"message"`
	CustomErrorCode int    `json:"customErrorCode,omitempty"`
}

//easyjson:json
type Errors []Error

func (e Errors) Contains(code int) bool {
	for _, err := range e {
		if err.Code == code {
			return true
		}
	}

	return false
}

//easyjson:json
type ErrorsResponse struct {
	Errors     Errors `json:"errors"`
	StatusCode int    `json:"status_code,omitempty"`
}

func (e ErrorsResponse) Error() string {
	buf, err := e.MarshalJSON()
	if err != nil {
		return fmt.Sprintf("error occurred while marshaling assetdelivery.Errors to JSON: %s", err.Error())
	}

	return string(buf)
}

//easyjson:json
type AssetRequestItem struct {
	RequestID string `json:"requestId"`
	AssetID   int64  `json:"assetId"`
}

//easyjson:json
type AssetRequestItems []AssetRequestItem

func AssetRequestItemsFromAssetIDs(ids ...int64) AssetRequestItems {
	items := make(AssetRequestItems, 0, len(ids))
	for _, id := range ids {
		items = append(items, AssetRequestItem{
			RequestID: uuid.NewString(),
			AssetID:   id,
		})
	}

	return items
}

//easyjson:json
type Location struct {
	AssetFormat string `json:"assetFormat" csv:"asset_format"`
	Location    string `json:"location" csv:"location"`
}

//easyjson:json
type Locations []Location

//easyjson:json
type AssetDescription struct {
	Locations            Locations `json:"locations" csv:"locations" csv[]:"1"`
	Errors               Errors    `json:"errors,omitempty" csv:"-"`
	RequestID            string    `json:"requestId" csv:"-"`
	IsHashDynamic        bool      `csv:"is_hash_dynamic"`
	IsCopyrightProtected bool      `csv:"is_copyright_protected"`
	IsArchived           bool      `json:"isArchived" csv:"is_archived"`
	AssetTypeID          int       `json:"assetTypeId" csv:"asset_type_id"`

	AssetID int64 `json:"assetId,omitempty" csv:"asset_id"`
}

func (a AssetDescription) Etag() string {
	return filepath.Base(a.Locations[0].Location)
}

//easyjson:json
type AssetDescriptions []AssetDescription

func (a AssetDescriptions) DiscardErrored() (filtered AssetDescriptions) {
	for i := range a {
		if a[i].Errors != nil {
			continue
		}

		filtered = append(filtered, a[i])
	}

	return
}

func (a AssetDescriptions) FilterByAssetType(assetType int) (filtered AssetDescriptions) {
	for i := range a {
		if a[i].AssetTypeID != assetType {
			continue
		}

		filtered = append(filtered, a[i])
	}

	return
}

func (a AssetDescriptions) DedupByEtag() (deduped AssetDescriptions) {
	alreadyFound := make(map[string]struct{})
	for i := range a {
		etag := filepath.Base(a[i].Etag())
		if _, ok := alreadyFound[etag]; ok {
			continue
		}
		alreadyFound[etag] = struct{}{}

		deduped = append(deduped, a[i])
	}

	return
}
