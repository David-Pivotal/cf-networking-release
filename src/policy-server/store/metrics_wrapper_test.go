package store_test

import (
	"errors"
	"policy-server/store"
	"policy-server/store/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetricsWrapper", func() {
	var (
		metricsWrapper    *store.MetricsWrapper
		policies          []store.Policy
		tags              []store.Tag
		srcGuids          []string
		destGuids         []string
		fakeMetricsSender *fakes.MetricsSender
		fakeStore         *fakes.Store
	)

	BeforeEach(func() {
		fakeStore = &fakes.Store{}
		fakeMetricsSender = &fakes.MetricsSender{}
		metricsWrapper = &store.MetricsWrapper{
			Store:         fakeStore,
			MetricsSender: fakeMetricsSender,
		}
		policies = []store.Policy{{
			Source: store.Source{ID: "some-app-guid"},
			Destination: store.Destination{
				ID:       "some-other-app-guid",
				Protocol: "tcp",
				Port:     8080,
			},
		}}
		tags = []store.Tag{{
			ID:  "some-app-guid",
			Tag: "0001",
		}, {
			ID:  "some-other-app-guid",
			Tag: "0002",
		}}
		srcGuids = []string{"some-app-guid"}
		destGuids = []string{"some-other-app-guid"}
	})

	Describe("Create", func() {
		It("calls Create on the Store", func() {
			err := metricsWrapper.Create(policies)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeStore.CreateCallCount()).To(Equal(1))
			Expect(fakeStore.CreateArgsForCall(0)).To(Equal(policies))
		})

		It("emits a metric", func() {
			err := metricsWrapper.Create(policies)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
			name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
			Expect(name).To(Equal("StoreCreateSuccessTime"))
		})

		Context("when there is an error", func() {
			BeforeEach(func() {
				fakeStore.CreateReturns(errors.New("banana"))
			})
			It("emits an error metric", func() {
				err := metricsWrapper.Create(policies)
				Expect(err).To(MatchError("banana"))

				Expect(fakeMetricsSender.IncrementCounterCallCount()).To(Equal(1))
				Expect(fakeMetricsSender.IncrementCounterArgsForCall(0)).To(Equal("StoreCreateError"))

				Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
				name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
				Expect(name).To(Equal("StoreCreateErrorTime"))
			})
		})
	})

	Describe("All", func() {
		BeforeEach(func() {
			fakeStore.AllReturns(policies, nil)
		})
		It("returns the result of All on the Store", func() {
			returnedPolicies, err := metricsWrapper.All()
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedPolicies).To(Equal(policies))

			Expect(fakeStore.AllCallCount()).To(Equal(1))
		})

		It("emits a metric", func() {
			_, err := metricsWrapper.All()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
			name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
			Expect(name).To(Equal("StoreAllSuccessTime"))
		})

		Context("when there is an error", func() {
			BeforeEach(func() {
				fakeStore.AllReturns(nil, errors.New("banana"))
			})
			It("emits an error metric", func() {
				_, err := metricsWrapper.All()
				Expect(err).To(MatchError("banana"))

				Expect(fakeMetricsSender.IncrementCounterCallCount()).To(Equal(1))
				Expect(fakeMetricsSender.IncrementCounterArgsForCall(0)).To(Equal("StoreAllError"))

				Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
				name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
				Expect(name).To(Equal("StoreAllErrorTime"))

			})
		})
	})

	Describe("ByGuids", func() {
		BeforeEach(func() {
			fakeStore.ByGuidsReturns(policies, nil)
		})
		It("returns the result of ByGuids on the Store", func() {
			returnedPolicies, err := metricsWrapper.ByGuids(srcGuids, destGuids)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedPolicies).To(Equal(policies))

			Expect(fakeStore.ByGuidsCallCount()).To(Equal(1))
			returnedSrcGuids, returnedDestGuids := fakeStore.ByGuidsArgsForCall(0)
			Expect(returnedSrcGuids).To(Equal(srcGuids))
			Expect(returnedDestGuids).To(Equal(destGuids))
		})

		It("emits a metric", func() {
			_, err := metricsWrapper.ByGuids(srcGuids, destGuids)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
			name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
			Expect(name).To(Equal("StoreByGuidsSuccessTime"))
		})

		Context("when there is an error", func() {
			BeforeEach(func() {
				fakeStore.ByGuidsReturns(nil, errors.New("banana"))
			})
			It("emits an error metric", func() {
				_, err := metricsWrapper.ByGuids(srcGuids, destGuids)
				Expect(err).To(MatchError("banana"))

				Expect(fakeMetricsSender.IncrementCounterCallCount()).To(Equal(1))
				Expect(fakeMetricsSender.IncrementCounterArgsForCall(0)).To(Equal("StoreByGuidsError"))

				Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
				name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
				Expect(name).To(Equal("StoreByGuidsErrorTime"))

			})
		})
	})

	Describe("CheckDatabase", func() {
		It("calls CheckDatabase on the Store", func() {
			err := metricsWrapper.CheckDatabase()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeStore.CheckDatabaseCallCount()).To(Equal(1))
		})

		It("emits a metric", func() {
			err := metricsWrapper.CheckDatabase()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
			name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
			Expect(name).To(Equal("StoreCheckDatabaseSuccessTime"))
		})

		Context("when there is an error", func() {
			BeforeEach(func() {
				fakeStore.CheckDatabaseReturns(errors.New("huckleberry"))
			})
			It("emits an error metric", func() {
				err := metricsWrapper.CheckDatabase()
				Expect(err).To(MatchError("huckleberry"))

				Expect(fakeMetricsSender.IncrementCounterCallCount()).To(Equal(1))
				Expect(fakeMetricsSender.IncrementCounterArgsForCall(0)).To(Equal("StoreCheckDatabaseError"))

				Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
				name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
				Expect(name).To(Equal("StoreCheckDatabaseErrorTime"))
			})
		})
	})

	Describe("Delete", func() {
		It("calls Delete on the Store", func() {
			err := metricsWrapper.Delete(policies)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeStore.DeleteCallCount()).To(Equal(1))
			Expect(fakeStore.DeleteArgsForCall(0)).To(Equal(policies))
		})

		It("emits a metric", func() {
			err := metricsWrapper.Delete(policies)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
			name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
			Expect(name).To(Equal("StoreDeleteSuccessTime"))
		})

		Context("when there is an error", func() {
			BeforeEach(func() {
				fakeStore.DeleteReturns(errors.New("banana"))
			})
			It("emits an error metric", func() {
				err := metricsWrapper.Delete(policies)
				Expect(err).To(MatchError("banana"))

				Expect(fakeMetricsSender.IncrementCounterCallCount()).To(Equal(1))
				Expect(fakeMetricsSender.IncrementCounterArgsForCall(0)).To(Equal("StoreDeleteError"))

				Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
				name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
				Expect(name).To(Equal("StoreDeleteErrorTime"))
			})
		})
	})

	Describe("Tags", func() {
		BeforeEach(func() {
			fakeStore.TagsReturns(tags, nil)
		})
		It("calls Tags on the Store", func() {
			returnedTags, err := metricsWrapper.Tags()
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedTags).To(Equal(tags))

			Expect(fakeStore.TagsCallCount()).To(Equal(1))
		})

		It("emits a metric", func() {
			_, err := metricsWrapper.Tags()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
			name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
			Expect(name).To(Equal("StoreTagsSuccessTime"))

		})

		Context("when there is an error", func() {
			BeforeEach(func() {
				fakeStore.TagsReturns(nil, errors.New("banana"))
			})
			It("emits an error metric", func() {
				_, err := metricsWrapper.Tags()
				Expect(err).To(MatchError("banana"))

				Expect(fakeMetricsSender.IncrementCounterCallCount()).To(Equal(1))
				Expect(fakeMetricsSender.IncrementCounterArgsForCall(0)).To(Equal("StoreTagsError"))

				Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
				name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
				Expect(name).To(Equal("StoreTagsErrorTime"))

			})
		})
	})
})
