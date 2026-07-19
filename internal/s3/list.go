package s3

import (
	"encoding/xml"
	"net/http"
	"net/url"
)

// defaultMaxKeys is the S3 default page size, echoed on every (always-empty)
// list response since no client-supplied max-keys is honoured.
const defaultMaxKeys = 1000

type listBucketResult struct {
	XMLName     xml.Name `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult"`
	Name        string   `xml:"Name"`
	Prefix      string   `xml:"Prefix"`
	MaxKeys     int      `xml:"MaxKeys"`
	IsTruncated bool     `xml:"IsTruncated"`
	KeyCount    int      `xml:"KeyCount"`
}

type deleteResult struct {
	XMLName xml.Name `xml:"http://s3.amazonaws.com/doc/2006-03-01/ DeleteResult"`
}

// respondListObjects writes an always-empty ListObjects/ListObjectsV2 result.
func (h *Handler) respondListObjects(w http.ResponseWriter, bucket string) {
	writeXML(w, http.StatusOK, listBucketResult{
		Name:    bucket,
		MaxKeys: defaultMaxKeys,
	})
}

// respondDeleteObject writes the 204 No Content response for a single-object
// delete no-op.
func (h *Handler) respondDeleteObject(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// respondDeleteObjects writes an always-empty DeleteObjects (batch) result.
func (h *Handler) respondDeleteObjects(w http.ResponseWriter) {
	writeXML(w, http.StatusOK, deleteResult{})
}

// listingParams are the query parameters whose presence (regardless of
// value) dispatches a GET request to ListObjects/V2.
var listingParams = []string{"list-type", "prefix", "delimiter", "marker", "continuation-token"}

// isListRequest reports whether query carries any listing parameter.
func isListRequest(query url.Values) bool {
	for _, p := range listingParams {
		if query.Has(p) {
			return true
		}
	}
	return false
}
