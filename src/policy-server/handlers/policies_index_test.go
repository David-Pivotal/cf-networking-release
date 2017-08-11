package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"policy-server/handlers"
	"policy-server/handlers/fakes"
	"policy-server/uaa_client"

	apifakes "policy-server/api/fakes"

	"code.cloudfoundry.org/lager"

	"policy-server/store"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Policies index handler", func() {
	var (
		allPolicies          []store.Policy
		byGuidsPolicies      []store.Policy
		byGuidsAPIPolicies   []store.Policy
		expectedResponseBody []byte
		filteredPolicies     []store.Policy
		request              *http.Request
		handler              *handlers.PoliciesIndex
		resp                 *httptest.ResponseRecorder
		fakeStore            *fakes.DataStore
		fakePolicyFilter     *fakes.PolicyFilter
		fakeErrorResponse    *fakes.ErrorResponse
		fakeMapper           *apifakes.PolicyMapper
		logger               *lagertest.TestLogger
		expectedLogger       lager.Logger
		token                uaa_client.CheckTokenResponse
	)

	BeforeEach(func() {
		expectedResponseBody = []byte("some-response")
		allPolicies = []store.Policy{{
			Source: store.Source{ID: "some-app-guid", Tag: "some-tag"},
			Destination: store.Destination{
				ID:       "some-other-app-guid",
				Tag:      "some-other-tag",
				Protocol: "tcp",
				Ports: store.Ports{
					Start: 8080,
					End:   8080,
				},
			},
		}, {
			Source: store.Source{ID: "another-app-guid"},
			Destination: store.Destination{
				ID:       "some-other-app-guid",
				Protocol: "udp",
				Ports: store.Ports{
					Start: 1234,
					End:   1234,
				},
			},
		}, {
			Source: store.Source{ID: "yet-another-app-guid"},
			Destination: store.Destination{
				ID:       "yet-another-app-guid",
				Protocol: "udp",
				Ports: store.Ports{
					Start: 5555,
					End:   5555,
				},
			},
		}}

		byGuidsPolicies = []store.Policy{{
			Source: store.Source{ID: "some-app-guid", Tag: "some-tag"},
			Destination: store.Destination{
				ID:       "some-other-app-guid",
				Tag:      "some-other-tag",
				Protocol: "tcp",
				Ports: store.Ports{
					Start: 8080,
					End:   8080,
				},
			},
		}, {
			Source: store.Source{ID: "another-app-guid"},
			Destination: store.Destination{
				ID:       "some-other-app-guid",
				Protocol: "udp",
				Ports: store.Ports{
					Start: 1234,
					End:   1234,
				},
			},
		}}

		byGuidsAPIPolicies = []store.Policy{{
			Source: store.Source{ID: "some-app-guid", Tag: "some-tag"},
			Destination: store.Destination{
				ID:       "some-other-app-guid",
				Tag:      "some-other-tag",
				Protocol: "tcp",
				Ports: store.Ports{
					Start: 8080,
					End:   8080,
				},
			},
		}, {
			Source: store.Source{ID: "another-app-guid"},
			Destination: store.Destination{
				ID:       "some-other-app-guid",
				Protocol: "udp",
				Ports: store.Ports{
					Start: 1234,
					End:   1234,
				},
			},
		}}

		filteredPolicies = []store.Policy{{
			Source: store.Source{ID: "some-app-guid", Tag: "some-tag"},
			Destination: store.Destination{
				ID:       "some-other-app-guid",
				Tag:      "some-other-tag",
				Protocol: "tcp",
				Ports: store.Ports{
					Start: 8080,
					End:   8080,
				},
			},
		}}

		var err error
		request, err = http.NewRequest("GET", "/networking/v0/external/policies", nil)
		Expect(err).NotTo(HaveOccurred())

		fakeStore = &fakes.DataStore{}
		fakeStore.AllReturns(allPolicies, nil)
		fakeStore.ByGuidsReturns(byGuidsPolicies, nil)
		fakeErrorResponse = &fakes.ErrorResponse{}
		fakePolicyFilter = &fakes.PolicyFilter{}
		fakePolicyFilter.FilterPoliciesStub = func(policies []store.Policy, userToken uaa_client.CheckTokenResponse) ([]store.Policy, error) {
			return filteredPolicies, nil
		}
		fakeMapper = &apifakes.PolicyMapper{}
		fakeMapper.AsBytesReturns(expectedResponseBody, nil)
		logger = lagertest.NewTestLogger("test")
		handler = &handlers.PoliciesIndex{
			Store:         fakeStore,
			Mapper:        fakeMapper,
			PolicyFilter:  fakePolicyFilter,
			ErrorResponse: fakeErrorResponse,
		}

		token = uaa_client.CheckTokenResponse{
			Scope:    []string{"some-scope", "some-other-scope"},
			UserID:   "some-user-id",
			UserName: "some-user",
		}
		resp = httptest.NewRecorder()

		expectedLogger = lager.NewLogger("test").Session("index-policies")

		testSink := lagertest.NewTestSink()
		expectedLogger.RegisterSink(testSink)
		expectedLogger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))
	})

	It("returns all the policies, but does not include the tags", func() {
		MakeRequestWithLoggerAndAuth(handler.ServeHTTP, resp, request, logger, token)

		Expect(fakeStore.AllCallCount()).To(Equal(1))
		Expect(fakePolicyFilter.FilterPoliciesCallCount()).To(Equal(1))
		Expect(resp.Code).To(Equal(http.StatusOK))
		Expect(resp.Body.Bytes()).To(Equal(expectedResponseBody))
	})

	Context("when the logger isn't on the request context", func() {
		It("still works", func() {
			MakeRequestWithAuth(handler.ServeHTTP, resp, request, token)

			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(resp.Body.Bytes()).To(Equal(expectedResponseBody))
		})
	})

	Context("when the token isn't on the request context", func() {
		It("still works", func() {
			MakeRequestWithLogger(handler.ServeHTTP, resp, request, logger)

			Expect(fakePolicyFilter.FilterPoliciesCallCount()).To(Equal(1))
			_, filterToken := fakePolicyFilter.FilterPoliciesArgsForCall(0)
			Expect(filterToken).To(Equal(uaa_client.CheckTokenResponse{}))
			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(resp.Body.Bytes()).To(Equal(expectedResponseBody))
		})
	})

	Context("when rendering the policies as bytes fails", func() {
		BeforeEach(func() {
			var err error
			request, err = http.NewRequest("GET", "/networking/v0/external/policies?id=some-app-guid,yet-another-app-guid", nil)
			Expect(err).NotTo(HaveOccurred())

			fakeMapper.AsBytesReturns(nil, errors.New("banana"))
		})

		It("calls the internal server error handler", func() {
			MakeRequestWithLoggerAndAuth(handler.ServeHTTP, resp, request, logger, token)

			Expect(fakeErrorResponse.InternalServerErrorCallCount()).To(Equal(1))

			l, w, err, description := fakeErrorResponse.InternalServerErrorArgsForCall(0)
			Expect(l).To(Equal(expectedLogger))
			Expect(w).To(Equal(resp))
			Expect(err).To(MatchError("banana"))
			Expect(description).To(Equal("map policy as bytes failed"))
		})
	})

	Context("when a list of ids is provided as a query parameter", func() {
		BeforeEach(func() {
			var err error
			request, err = http.NewRequest("GET", "/networking/v0/external/policies?id=some-app-guid,yet-another-app-guid", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("filters on only those policies returned by ByGuids", func() {
			MakeRequestWithLoggerAndAuth(handler.ServeHTTP, resp, request, logger, token)

			Expect(fakeStore.ByGuidsCallCount()).To(Equal(1))
			srcGuids, dstGuids := fakeStore.ByGuidsArgsForCall(0)
			Expect(srcGuids).To(ConsistOf([]string{"some-app-guid", "yet-another-app-guid"}))
			Expect(dstGuids).To(ConsistOf([]string{"some-app-guid", "yet-another-app-guid"}))
			Expect(fakePolicyFilter.FilterPoliciesCallCount()).To(Equal(1))
			policies, userToken := fakePolicyFilter.FilterPoliciesArgsForCall(0)
			Expect(policies).To(Equal(byGuidsAPIPolicies))
			Expect(userToken).To(Equal(token))
			Expect(resp.Code).To(Equal(http.StatusOK))
		})

		Context("when the id list is empty", func() {
			It("filters on only those policies returned by ByGuids", func() {
				var err error
				request, err = http.NewRequest("GET", "/networking/v0/external/policies?id=", nil)
				Expect(err).NotTo(HaveOccurred())

				MakeRequestWithLoggerAndAuth(handler.ServeHTTP, resp, request, logger, token)
				Expect(fakeStore.ByGuidsCallCount()).To(Equal(1))
				srcGuids, destGuids := fakeStore.ByGuidsArgsForCall(0)
				Expect(srcGuids).To(Equal([]string{""}))
				Expect(destGuids).To(Equal([]string{""}))
				Expect(fakePolicyFilter.FilterPoliciesCallCount()).To(Equal(1))
				policies, userToken := fakePolicyFilter.FilterPoliciesArgsForCall(0)
				Expect(policies).To(Equal(byGuidsAPIPolicies))
				Expect(userToken).To(Equal(token))

				Expect(resp.Code).To(Equal(http.StatusOK))
			})
		})
	})

	Context("when the store throws an error", func() {
		BeforeEach(func() {
			fakeStore.AllReturns(nil, errors.New("banana"))
		})

		It("calls the internal server error handler", func() {
			MakeRequestWithLoggerAndAuth(handler.ServeHTTP, resp, request, logger, token)

			Expect(fakeErrorResponse.InternalServerErrorCallCount()).To(Equal(1))

			l, w, err, description := fakeErrorResponse.InternalServerErrorArgsForCall(0)
			Expect(l).To(Equal(expectedLogger))
			Expect(w).To(Equal(resp))
			Expect(err).To(MatchError("banana"))
			Expect(description).To(Equal("database read failed"))
		})
	})

	Context("when the policy filter throws an error", func() {
		BeforeEach(func() {
			fakePolicyFilter.FilterPoliciesReturns(nil, errors.New("banana"))
		})

		It("calls the internal server error handler", func() {
			MakeRequestWithLoggerAndAuth(handler.ServeHTTP, resp, request, logger, token)

			Expect(fakeErrorResponse.InternalServerErrorCallCount()).To(Equal(1))

			l, w, err, description := fakeErrorResponse.InternalServerErrorArgsForCall(0)
			Expect(l).To(Equal(expectedLogger))
			Expect(w).To(Equal(resp))
			Expect(err).To(MatchError("banana"))
			Expect(description).To(Equal("filter policies failed"))
		})
	})
})
