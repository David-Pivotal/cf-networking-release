package handlers_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"policy-server/handlers"
	"policy-server/handlers/fakes"
	"policy-server/uaa_client"

	apifakes "policy-server/api/fakes"

	"code.cloudfoundry.org/cf-networking-helpers/testsupport"
	"code.cloudfoundry.org/lager"

	"policy-server/store"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PoliciesDelete", func() {
	var (
		requestBody       string
		request           *http.Request
		handler           *handlers.PoliciesDelete
		resp              *httptest.ResponseRecorder
		fakeStore         *fakes.DataStore
		fakeMapper        *apifakes.PolicyMapper
		logger            *lagertest.TestLogger
		expectedPolicies  []store.Policy
		fakeValidator     *fakes.Validator
		fakePolicyGuard   *fakes.PolicyGuard
		fakeErrorResponse *fakes.ErrorResponse
		tokenData         uaa_client.CheckTokenResponse
	)

	const Route = "/networking/v0/external/policies/delete"

	BeforeEach(func() {
		var err error
		requestBody = "some request body"
		request, err = http.NewRequest("POST", Route, bytes.NewBuffer([]byte(requestBody)))
		Expect(err).NotTo(HaveOccurred())

		fakeStore = &fakes.DataStore{}
		fakeMapper = &apifakes.PolicyMapper{}
		fakeValidator = &fakes.Validator{}
		fakePolicyGuard = &fakes.PolicyGuard{}
		logger = lagertest.NewTestLogger("test")
		fakeErrorResponse = &fakes.ErrorResponse{}
		handler = &handlers.PoliciesDelete{
			Mapper:        fakeMapper,
			Store:         fakeStore,
			Validator:     fakeValidator,
			PolicyGuard:   fakePolicyGuard,
			ErrorResponse: fakeErrorResponse,
		}
		resp = httptest.NewRecorder()

		expectedPolicies = []store.Policy{{
			Source: store.Source{ID: "some-app-guid"},
			Destination: store.Destination{
				ID:       "some-other-app-guid",
				Protocol: "tcp",
				Ports: store.Ports{
					Start: 8080,
					End:   8080,
				},
			},
		}}

		tokenData = uaa_client.CheckTokenResponse{
			Scope:    []string{"network.admin"},
			UserName: "some_user",
		}
		fakeMapper.AsStorePolicyReturns(expectedPolicies, nil)
		fakePolicyGuard.CheckAccessReturns(true, nil)
	})

	It("removes the entry from the policy server", func() {
		handler.ServeHTTP(logger, resp, request, tokenData)

		Expect(fakeMapper.AsStorePolicyCallCount()).To(Equal(1))
		Expect(fakeMapper.AsStorePolicyArgsForCall(0)).To(Equal([]byte(requestBody)))

		Expect(fakePolicyGuard.CheckAccessCallCount()).To(Equal(1))
		policies, token := fakePolicyGuard.CheckAccessArgsForCall(0)
		Expect(policies).To(Equal(expectedPolicies))
		Expect(token).To(Equal(tokenData))
		Expect(fakeStore.DeleteCallCount()).To(Equal(1))
		Expect(fakeStore.DeleteArgsForCall(0)).To(Equal(expectedPolicies))
		Expect(resp.Code).To(Equal(http.StatusOK))
		Expect(resp.Body.String()).To(MatchJSON("{}"))
	})

	It("logs the policy with username and app guid", func() {
		handler.ServeHTTP(logger, resp, request, tokenData)
		Expect(logger.Logs()).To(HaveLen(1))
		Expect(logger.Logs()[0]).To(SatisfyAll(
			LogsWith(lager.INFO, "test.delete-policies.deleted-policies"),
			HaveLogData(SatisfyAll(
				HaveLen(3),
				HaveKeyWithValue("policies", SatisfyAll(
					HaveLen(1),
					ConsistOf(
						SatisfyAll(
							HaveKeyWithValue("Source", HaveKeyWithValue("ID", "some-app-guid")),
							HaveKeyWithValue("Destination", SatisfyAll(
								HaveKeyWithValue("ID", "some-other-app-guid"),
								HaveKeyWithValue("Protocol", "tcp"),
								HaveKeyWithValue("Ports", SatisfyAll(
									HaveLen(2),
									HaveKeyWithValue("Start", BeEquivalentTo(8080)),
									HaveKeyWithValue("End", BeEquivalentTo(8080)),
								)),
							)),
						),
					),
				)),
				HaveKeyWithValue("session", "1"),
				HaveKeyWithValue("userName", "some_user"),
			)),
		))
	})

	Context("when the mapper fails to get store policies", func() {
		BeforeEach(func() {
			fakeMapper.AsStorePolicyReturns(nil, errors.New("banana"))
		})
		It("calls the bad request header, and logs the error", func() {
			handler.ServeHTTP(logger, resp, request, tokenData)

			Expect(fakeErrorResponse.BadRequestCallCount()).To(Equal(1))

			w, err, message, description := fakeErrorResponse.BadRequestArgsForCall(0)
			Expect(w).To(Equal(resp))
			Expect(err).To(MatchError("banana"))
			Expect(message).To(Equal("delete-policies"))
			Expect(description).To(Equal("could not map request to store policies: banana"))

			By("logging the error")
			Expect(logger.Logs()).To(HaveLen(1))
			Expect(logger.Logs()[0]).To(SatisfyAll(
				LogsWith(lager.ERROR, "test.delete-policies.failed-mapping-policies"),
				HaveLogData(SatisfyAll(
					HaveLen(2),
					HaveKeyWithValue("error", "banana"),
					HaveKeyWithValue("session", "1"),
				)),
			))
		})
	})

	Context("when the policy guard returns false", func() {
		BeforeEach(func() {
			fakePolicyGuard.CheckAccessReturns(false, nil)
		})

		It("calls the forbidden handler", func() {
			handler.ServeHTTP(logger, resp, request, tokenData)

			Expect(fakeErrorResponse.ForbiddenCallCount()).To(Equal(1))

			w, err, message, description := fakeErrorResponse.ForbiddenArgsForCall(0)
			Expect(w).To(Equal(resp))
			Expect(err).To(MatchError("one or more applications cannot be found or accessed"))
			Expect(message).To(Equal("delete-policies"))
			Expect(description).To(Equal("one or more applications cannot be found or accessed"))

			By("logging the error")
			Expect(logger.Logs()).To(HaveLen(1))
			Expect(logger.Logs()[0]).To(SatisfyAll(
				LogsWith(lager.ERROR, "test.delete-policies.failed-authorizing-access"),
				HaveLogData(SatisfyAll(
					HaveLen(2),
					HaveKeyWithValue("error", "one or more applications cannot be found or accessed"),
					HaveKeyWithValue("session", "1"),
				)),
			))
		})
	})

	Context("when the policy guard returns an error", func() {
		BeforeEach(func() {
			fakePolicyGuard.CheckAccessReturns(false, errors.New("banana"))
		})

		It("calls the internal server error handler", func() {
			handler.ServeHTTP(logger, resp, request, tokenData)

			Expect(fakeErrorResponse.InternalServerErrorCallCount()).To(Equal(1))

			w, err, message, description := fakeErrorResponse.InternalServerErrorArgsForCall(0)
			Expect(w).To(Equal(resp))
			Expect(err).To(MatchError("banana"))
			Expect(message).To(Equal("delete-policies"))
			Expect(description).To(Equal("check access failed"))

			By("logging the error")
			Expect(logger.Logs()).To(HaveLen(1))
			Expect(logger.Logs()[0]).To(SatisfyAll(
				LogsWith(lager.ERROR, "test.delete-policies.failed-checking-access"),
				HaveLogData(SatisfyAll(
					HaveLen(2),
					HaveKeyWithValue("error", "banana"),
					HaveKeyWithValue("session", "1"),
				)),
			))
		})
	})

	Context("when a policy to delete includes any validation error", func() {
		BeforeEach(func() {
			fakeValidator.ValidatePoliciesReturns(errors.New("banana"))
		})

		It("calls the bad request handler", func() {
			handler.ServeHTTP(logger, resp, request, tokenData)

			Expect(fakeErrorResponse.BadRequestCallCount()).To(Equal(1))

			w, err, message, description := fakeErrorResponse.BadRequestArgsForCall(0)
			Expect(w).To(Equal(resp))
			Expect(err).To(MatchError("banana"))
			Expect(message).To(Equal("delete-policies"))
			Expect(description).To(Equal("banana"))

			By("logging the error")
			Expect(logger.Logs()).To(HaveLen(1))
			Expect(logger.Logs()[0]).To(SatisfyAll(
				LogsWith(lager.ERROR, "test.delete-policies.failed-validating-policies"),
				HaveLogData(SatisfyAll(
					HaveLen(2),
					HaveKeyWithValue("error", "banana"),
					HaveKeyWithValue("session", "1"),
				)),
			))
		})
	})

	Context("when reading the request body fails", func() {
		BeforeEach(func() {
			request.Body = &testsupport.BadReader{}
		})

		It("calls the bad request handler", func() {
			handler.ServeHTTP(logger, resp, request, tokenData)

			Expect(fakeErrorResponse.BadRequestCallCount()).To(Equal(1))

			w, err, message, description := fakeErrorResponse.BadRequestArgsForCall(0)
			Expect(w).To(Equal(resp))
			Expect(err).To(MatchError("banana"))
			Expect(message).To(Equal("delete-policies"))
			Expect(description).To(Equal("invalid request body"))

			By("logging the error")
			Expect(logger.Logs()).To(HaveLen(1))
			Expect(logger.Logs()[0]).To(SatisfyAll(
				LogsWith(lager.ERROR, "test.delete-policies.failed-reading-request-body"),
				HaveLogData(SatisfyAll(
					HaveLen(2),
					HaveKeyWithValue("error", "banana"),
					HaveKeyWithValue("session", "1"),
				)),
			))
		})
	})

	Context("when deleting from the store fails", func() {
		BeforeEach(func() {
			fakeStore.DeleteReturns(errors.New("banana"))
		})

		It("calls the internal server error handler", func() {
			handler.ServeHTTP(logger, resp, request, tokenData)

			Expect(fakeErrorResponse.InternalServerErrorCallCount()).To(Equal(1))

			w, err, message, description := fakeErrorResponse.InternalServerErrorArgsForCall(0)
			Expect(w).To(Equal(resp))
			Expect(err).To(MatchError("banana"))
			Expect(message).To(Equal("delete-policies"))
			Expect(description).To(Equal("database delete failed"))

			By("logging the error")
			Expect(logger.Logs()).To(HaveLen(1))
			Expect(logger.Logs()[0]).To(SatisfyAll(
				LogsWith(lager.ERROR, "test.delete-policies.failed-deleting-in-database"),
				HaveLogData(SatisfyAll(
					HaveLen(2),
					HaveKeyWithValue("error", "banana"),
					HaveKeyWithValue("session", "1"),
				)),
			))
		})
	})
})
