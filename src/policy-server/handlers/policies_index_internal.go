package handlers

import (
	"net/http"
	"net/url"
	"policy-server/api"
	"policy-server/store"
	"strings"

	"code.cloudfoundry.org/cf-networking-helpers/marshal"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter -o fakes/data_store.go --fake-name DataStore . dataStore
type dataStore interface {
	All() ([]store.Policy, error)
	Create([]store.Policy) error
	Delete([]store.Policy) error
	Tags() ([]store.Tag, error)
	ByGuids([]string, []string) ([]store.Policy, error)
	CheckDatabase() error
}

type PoliciesIndexInternal struct {
	Logger        lager.Logger
	Store         dataStore
	Marshaler     marshal.Marshaler
	ErrorResponse errorResponse
}

func (h *PoliciesIndexInternal) ServeHTTP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("index-policies-internal")

	queryValues := req.URL.Query()
	ids := parseIds(queryValues)

	var policies []store.Policy
	var err error
	if len(ids) == 0 {
		policies, err = h.Store.All()
	} else {
		policies, err = h.Store.ByGuids(ids, ids)
	}

	if err != nil {
		logger.Error("failed-reading-database", err)
		h.ErrorResponse.InternalServerError(w, err, "policies-index-internal", "database read failed")
		return
	}

	policyResponse := struct {
		Policies []api.Policy `json:"policies"`
	}{api.MapStorePolicies(policies)}
	bytes, err := h.Marshaler.Marshal(policyResponse)
	if err != nil {
		logger.Error("failed-marshalling-policies", err)
		h.ErrorResponse.InternalServerError(w, err, "policies-index-internal", "database marshalling failed")
		return
	}

	w.Write(bytes)
}

func parseIds(queryValues url.Values) []string {
	var ids []string
	idList, ok := queryValues["id"]
	if ok {
		ids = strings.Split(idList[0], ",")
	}
	return ids
}
