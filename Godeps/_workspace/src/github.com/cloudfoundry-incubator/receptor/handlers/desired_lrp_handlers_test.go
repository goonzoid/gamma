package handlers_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/receptor/handlers"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs/fake_bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/cloudfoundry/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
)

var _ = Describe("Desired LRP Handlers", func() {
	var (
		logger           lager.Logger
		fakeBBS          *fake_bbs.FakeReceptorBBS
		responseRecorder *httptest.ResponseRecorder
		handler          *handlers.DesiredLRPHandler
	)

	BeforeEach(func() {
		fakeBBS = new(fake_bbs.FakeReceptorBBS)
		logger = lager.NewLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))
		responseRecorder = httptest.NewRecorder()
		handler = handlers.NewDesiredLRPHandler(fakeBBS, logger)
	})

	Describe("Create", func() {
		validCreateLRPRequest := receptor.DesiredLRPCreateRequest{
			ProcessGuid: "the-process-guid",
			Domain:      "the-domain",
			Stack:       "the-stack",
			RootFSPath:  "the-rootfs-path",
			Instances:   1,
			Action: &models.RunAction{
				Path: "the-path",
			},
		}

		expectedDesiredLRP := models.DesiredLRP{
			ProcessGuid: "the-process-guid",
			Domain:      "the-domain",
			Stack:       "the-stack",
			RootFSPath:  "the-rootfs-path",
			Instances:   1,
			Action: &models.RunAction{
				Path: "the-path",
			},
		}

		Context("when everything succeeds", func() {
			BeforeEach(func(done Done) {
				defer close(done)
				handler.Create(responseRecorder, newTestRequest(validCreateLRPRequest))
			})

			It("calls DesireLRP on the BBS", func() {
				Ω(fakeBBS.DesireLRPCallCount()).Should(Equal(1))
				desired := fakeBBS.DesireLRPArgsForCall(0)
				Ω(desired).To(Equal(expectedDesiredLRP))
			})

			It("responds with 201 CREATED", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusCreated))
			})

			It("responds with an empty body", func() {
				Ω(responseRecorder.Body.String()).Should(Equal(""))
			})
		})

		Context("when the BBS responds with an error", func() {
			BeforeEach(func(done Done) {
				defer close(done)
				fakeBBS.DesireLRPReturns(errors.New("ka-boom"))
				handler.Create(responseRecorder, newTestRequest(validCreateLRPRequest))
			})

			It("calls DesireLRP on the BBS", func() {
				Ω(fakeBBS.DesireLRPCallCount()).Should(Equal(1))
				desired := fakeBBS.DesireLRPArgsForCall(0)
				Ω(desired).To(Equal(expectedDesiredLRP))
			})

			It("responds with 500 INTERNAL ERROR", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusInternalServerError))
			})

			It("responds with a relevant error message", func() {
				expectedBody, _ := json.Marshal(receptor.Error{
					Type:    receptor.UnknownError,
					Message: "ka-boom",
				})

				Ω(responseRecorder.Body.String()).Should(Equal(string(expectedBody)))
			})
		})

		Context("when the desired LRP is invalid", func() {
			var validationError = models.ValidationError{}

			BeforeEach(func(done Done) {
				fakeBBS.DesireLRPReturns(validationError)

				defer close(done)
				handler.Create(responseRecorder, newTestRequest(validCreateLRPRequest))
			})

			It("responds with 400 BAD REQUEST", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusBadRequest))
			})

			It("responds with a relevant error message", func() {
				expectedBody, _ := json.Marshal(receptor.Error{
					Type:    receptor.InvalidLRP,
					Message: validationError.Error(),
				})
				Ω(responseRecorder.Body.String()).Should(Equal(string(expectedBody)))
			})
		})

		Context("when the request does not contain a DesiredLRPCreateRequest", func() {
			var garbageRequest = []byte(`farewell`)

			BeforeEach(func(done Done) {
				defer close(done)
				handler.Create(responseRecorder, newTestRequest(garbageRequest))
			})

			It("does not call DesireLRP on the BBS", func() {
				Ω(fakeBBS.DesireLRPCallCount()).Should(Equal(0))
			})

			It("responds with 400 BAD REQUEST", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusBadRequest))
			})

			It("responds with a relevant error message", func() {
				err := json.Unmarshal(garbageRequest, &receptor.DesiredLRPCreateRequest{})
				expectedBody, _ := json.Marshal(receptor.Error{
					Type:    receptor.InvalidJSON,
					Message: err.Error(),
				})
				Ω(responseRecorder.Body.String()).Should(Equal(string(expectedBody)))
			})
		})
	})

	Describe("Get", func() {
		var req *http.Request

		BeforeEach(func() {
			req = newTestRequest("")
			req.Form = url.Values{":process_guid": []string{"process-guid-0"}}
		})

		JustBeforeEach(func() {
			handler.Get(responseRecorder, req)
		})

		Context("when reading tasks from BBS succeeds", func() {
			BeforeEach(func() {
				fakeBBS.DesiredLRPByProcessGuidReturns(&models.DesiredLRP{
					ProcessGuid: "process-guid-0",
					Domain:      "domain-1",
					Action: &models.RunAction{
						Path: "the-path",
					},
				}, nil)
			})

			It("calls DesiredLRPByProcessGuid on the BBS", func() {
				Ω(fakeBBS.DesiredLRPByProcessGuidCallCount()).Should(Equal(1))
				Ω(fakeBBS.DesiredLRPByProcessGuidArgsForCall(0)).Should(Equal("process-guid-0"))
			})

			It("responds with 200 Status OK", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusOK))
			})

			It("returns a desired lrp response", func() {
				response := receptor.DesiredLRPResponse{}
				err := json.Unmarshal(responseRecorder.Body.Bytes(), &response)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(response.ProcessGuid).Should(Equal("process-guid-0"))
			})
		})

		Context("when reading from the BBS fails", func() {
			BeforeEach(func() {
				fakeBBS.DesiredLRPByProcessGuidReturns(nil, errors.New("Something went wrong"))
			})

			It("responds with an error", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusInternalServerError))
			})
		})

		Context("when the BBS returns a nil lrp", func() {
			BeforeEach(func() {
				fakeBBS.DesiredLRPByProcessGuidReturns(nil, nil)
			})

			It("responds with 404 Status NOT FOUND", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusNotFound))
			})

			It("returns an LRPNotFound error", func() {
				var responseError receptor.Error
				err := json.Unmarshal(responseRecorder.Body.Bytes(), &responseError)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(responseError).Should(Equal(receptor.Error{
					Type:    receptor.DesiredLRPNotFound,
					Message: "Desired LRP with guid 'process-guid-0' not found",
				}))
			})
		})

		Context("when the BBS reports no lrp found", func() {
			BeforeEach(func() {
				fakeBBS.DesiredLRPByProcessGuidReturns(nil, storeadapter.ErrorKeyNotFound)
			})

			It("responds with 404 Status NOT FOUND", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusNotFound))
			})

			It("returns an LRPNotFound error", func() {
				var responseError receptor.Error
				err := json.Unmarshal(responseRecorder.Body.Bytes(), &responseError)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(responseError).Should(Equal(receptor.Error{
					Type:    receptor.DesiredLRPNotFound,
					Message: "Desired LRP with guid 'process-guid-0' not found",
				}))
			})
		})

		Context("when the process guid is not provided", func() {
			BeforeEach(func() {
				req.Form = url.Values{}
			})

			It("does not call DesiredLRPByProcessGuid on the BBS", func() {
				Ω(fakeBBS.DesiredLRPByProcessGuidCallCount()).Should(Equal(0))
			})

			It("responds with 400 BAD REQUEST", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusBadRequest))
			})

			It("returns an unknown error", func() {
				var responseError receptor.Error
				err := json.Unmarshal(responseRecorder.Body.Bytes(), &responseError)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(responseError).Should(Equal(receptor.Error{
					Type:    receptor.InvalidRequest,
					Message: "process_guid missing from request",
				}))
			})
		})
	})

	Describe("Update", func() {
		expectedProcessGuid := "some-guid"
		instances := 15
		annotation := "new-annotation"
		routes := []string{"new-route-1", "new-route-2"}

		validUpdateRequest := receptor.DesiredLRPUpdateRequest{
			Instances:  &instances,
			Annotation: &annotation,
			Routes:     routes,
		}

		expectedUpdate := models.DesiredLRPUpdate{
			Instances:  &instances,
			Annotation: &annotation,
			Routes:     routes,
		}

		var req *http.Request

		BeforeEach(func() {
			req = newTestRequest(validUpdateRequest)
			req.Form = url.Values{":process_guid": []string{expectedProcessGuid}}
		})

		Context("when everything succeeds", func() {
			BeforeEach(func(done Done) {
				defer close(done)
				handler.Update(responseRecorder, req)
			})

			It("calls UpdateDesiredLRP on the BBS", func() {
				Ω(fakeBBS.UpdateDesiredLRPCallCount()).Should(Equal(1))
				processGuid, update := fakeBBS.UpdateDesiredLRPArgsForCall(0)
				Ω(processGuid).Should(Equal(expectedProcessGuid))
				Ω(update).Should(Equal(expectedUpdate))
			})

			It("responds with 204 NO CONTENT", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusNoContent))
			})

			It("responds with an empty body", func() {
				Ω(responseRecorder.Body.String()).Should(Equal(""))
			})
		})

		Context("when the :process_guid is blank", func() {
			BeforeEach(func() {
				req = newTestRequest(validUpdateRequest)
				handler.Update(responseRecorder, req)
			})

			It("does not call UpdateDesiredLRP on the BBS", func() {
				Ω(fakeBBS.UpdateDesiredLRPCallCount()).Should(Equal(0))
			})

			It("responds with 400 BAD REQUEST", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusBadRequest))
			})

			It("responds with a relevant error message", func() {
				expectedBody, _ := json.Marshal(receptor.Error{
					Type:    receptor.InvalidRequest,
					Message: "process_guid missing from request",
				})

				Ω(responseRecorder.Body.String()).Should(Equal(string(expectedBody)))
			})
		})

		Context("when the BBS responds with an error", func() {
			BeforeEach(func(done Done) {
				defer close(done)
				fakeBBS.UpdateDesiredLRPReturns(errors.New("ka-boom"))
				handler.Update(responseRecorder, req)
			})

			It("calls UpdateDesiredLRP on the BBS", func() {
				Ω(fakeBBS.UpdateDesiredLRPCallCount()).Should(Equal(1))
				processGuid, update := fakeBBS.UpdateDesiredLRPArgsForCall(0)
				Ω(processGuid).Should(Equal(expectedProcessGuid))
				Ω(update).Should(Equal(expectedUpdate))
			})

			It("responds with 500 INTERNAL ERROR", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusInternalServerError))
			})

			It("responds with a relevant error message", func() {
				expectedBody, _ := json.Marshal(receptor.Error{
					Type:    receptor.UnknownError,
					Message: "ka-boom",
				})

				Ω(responseRecorder.Body.String()).Should(Equal(string(expectedBody)))
			})
		})

		Context("when the BBS indicates the LRP was not found", func() {
			BeforeEach(func(done Done) {
				defer close(done)
				fakeBBS.UpdateDesiredLRPReturns(storeadapter.ErrorKeyNotFound)
				handler.Update(responseRecorder, req)
			})

			It("calls UpdateDesiredLRP on the BBS", func() {
				Ω(fakeBBS.UpdateDesiredLRPCallCount()).Should(Equal(1))
				processGuid, update := fakeBBS.UpdateDesiredLRPArgsForCall(0)
				Ω(processGuid).Should(Equal(expectedProcessGuid))
				Ω(update).Should(Equal(expectedUpdate))
			})

			It("responds with 404 NOT FOUND", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusNotFound))
			})

			It("responds with a relevant error message", func() {
				expectedBody, _ := json.Marshal(receptor.Error{
					Type:    receptor.DesiredLRPNotFound,
					Message: "Desired LRP with guid 'some-guid' not found",
				})

				Ω(responseRecorder.Body.String()).Should(Equal(string(expectedBody)))
			})
		})

		Context("when the request does not contain an DesiredLRPUpdateRequest", func() {
			var garbageRequest = []byte(`farewell`)

			BeforeEach(func(done Done) {
				defer close(done)
				req = newTestRequest(garbageRequest)
				req.Form = url.Values{":process_guid": []string{expectedProcessGuid}}
				handler.Update(responseRecorder, req)
			})

			It("does not call DesireLRP on the BBS", func() {
				Ω(fakeBBS.UpdateDesiredLRPCallCount()).Should(Equal(0))
			})

			It("responds with 400 BAD REQUEST", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusBadRequest))
			})

			It("responds with a relevant error message", func() {
				err := json.Unmarshal(garbageRequest, &receptor.DesiredLRPUpdateRequest{})
				expectedBody, _ := json.Marshal(receptor.Error{
					Type:    receptor.InvalidJSON,
					Message: err.Error(),
				})
				Ω(responseRecorder.Body.String()).Should(Equal(string(expectedBody)))
			})
		})
	})

	Describe("Delete", func() {
		var req *http.Request

		BeforeEach(func() {
			req = newTestRequest("")
			req.Form = url.Values{":process_guid": []string{"process-guid-0"}}
		})

		JustBeforeEach(func() {
			handler.Delete(responseRecorder, req)
		})

		Context("when deleting lrp from BBS succeeds", func() {
			BeforeEach(func() {
				fakeBBS.RemoveDesiredLRPByProcessGuidReturns(nil)
			})

			It("calls the BBS to remove the desired LRP", func() {
				Ω(fakeBBS.RemoveDesiredLRPByProcessGuidCallCount()).Should(Equal(1))
				Ω(fakeBBS.RemoveDesiredLRPByProcessGuidArgsForCall(0)).Should(Equal("process-guid-0"))
			})

			It("responds with 204 NO CONTENT", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusNoContent))
			})

			It("returns no body", func() {
				Ω(responseRecorder.Body.Bytes()).Should(BeEmpty())
			})
		})

		Context("when reading from the BBS fails", func() {
			BeforeEach(func() {
				fakeBBS.RemoveDesiredLRPByProcessGuidReturns(errors.New("Something went wrong"))
			})

			It("responds with 500 INTERNAL SERVER ERROR", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusInternalServerError))
			})

			It("provides relevant error information", func() {
				var deleteError receptor.Error
				err := json.Unmarshal(responseRecorder.Body.Bytes(), &deleteError)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(deleteError).Should(Equal(receptor.Error{
					Type:    receptor.UnknownError,
					Message: "Something went wrong",
				}))
			})
		})

		Context("when the BBS returns no lrp", func() {
			BeforeEach(func() {
				fakeBBS.RemoveDesiredLRPByProcessGuidReturns(storeadapter.ErrorKeyNotFound)
			})

			It("responds with 404 Status NOT FOUND", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusNotFound))
			})

			It("returns an LRPNotFound error", func() {
				var responseError receptor.Error
				err := json.Unmarshal(responseRecorder.Body.Bytes(), &responseError)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(responseError).Should(Equal(receptor.Error{
					Type:    receptor.DesiredLRPNotFound,
					Message: "Desired LRP with guid 'process-guid-0' not found",
				}))
			})
		})

		Context("when the process guid is not provided", func() {
			BeforeEach(func() {
				req.Form = url.Values{}
			})

			It("does not call the BBS to remove the desired LRP", func() {
				Ω(fakeBBS.RemoveDesiredLRPByProcessGuidCallCount()).Should(Equal(0))
			})

			It("responds with 400 BAD REQUEST", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusBadRequest))
			})

			It("returns an unknown error", func() {
				var responseError receptor.Error
				err := json.Unmarshal(responseRecorder.Body.Bytes(), &responseError)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(responseError).Should(Equal(receptor.Error{
					Type:    receptor.InvalidRequest,
					Message: "process_guid missing from request",
				}))
			})
		})
	})

	Describe("GetAll", func() {
		JustBeforeEach(func() {
			handler.GetAll(responseRecorder, newTestRequest(""))
		})

		Context("when reading LRPs from BBS succeeds", func() {
			BeforeEach(func() {
				fakeBBS.DesiredLRPsReturns([]models.DesiredLRP{
					{
						ProcessGuid: "process-guid-0",
						Domain:      "domain-1",
						Action: &models.RunAction{
							Path: "the-path",
						},
					},
					{
						ProcessGuid: "process-guid-1",
						Domain:      "domain-1",
						Action: &models.RunAction{
							Path: "the-path",
						},
					},
				}, nil)
			})

			It("call the BBS to retrieve the desired LRP", func() {
				Ω(fakeBBS.DesiredLRPsCallCount()).Should(Equal(1))
			})

			It("responds with 200 Status OK", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusOK))
			})

			It("returns a list of desired lrp responses", func() {
				response := []receptor.DesiredLRPResponse{}
				err := json.Unmarshal(responseRecorder.Body.Bytes(), &response)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(response).Should(HaveLen(2))
				Ω(response[0].ProcessGuid).Should(Equal("process-guid-0"))
				Ω(response[1].ProcessGuid).Should(Equal("process-guid-1"))
			})
		})

		Context("when the BBS returns no lrps", func() {
			BeforeEach(func() {
				fakeBBS.DesiredLRPsReturns([]models.DesiredLRP{}, nil)
			})

			It("call the BBS to retrieve the desired LRP", func() {
				Ω(fakeBBS.DesiredLRPsCallCount()).Should(Equal(1))
			})

			It("responds with 200 Status OK", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusOK))
			})

			It("returns an empty list", func() {
				Ω(responseRecorder.Body.String()).Should(Equal("[]"))
			})
		})

		Context("when reading from the BBS fails", func() {
			BeforeEach(func() {
				fakeBBS.DesiredLRPsReturns([]models.DesiredLRP{}, errors.New("Something went wrong"))
			})

			It("responds with an error", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusInternalServerError))
			})

			It("provides relevant error information", func() {
				var receptorError receptor.Error
				err := json.Unmarshal(responseRecorder.Body.Bytes(), &receptorError)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(receptorError).Should(Equal(receptor.Error{
					Type:    receptor.UnknownError,
					Message: "Something went wrong",
				}))
			})
		})
	})

	Describe("GetAllByDomain", func() {
		var req *http.Request

		BeforeEach(func() {
			req = newTestRequest("")
			req.Form = url.Values{":domain": []string{"domain-1"}}
		})

		JustBeforeEach(func() {
			handler.GetAllByDomain(responseRecorder, req)
		})

		Context("when reading LRPs by domain from BBS succeeds", func() {
			BeforeEach(func() {
				fakeBBS.DesiredLRPsByDomainReturns([]models.DesiredLRP{
					{
						ProcessGuid: "process-guid-0",
						Domain:      "domain-1",
						Action: &models.RunAction{
							Path: "the-path",
						},
					},
					{
						ProcessGuid: "process-guid-1",
						Domain:      "domain-1",
						Action: &models.RunAction{
							Path: "the-path",
						},
					},
				}, nil)
			})

			It("call the BBS to retrieve the desired LRP", func() {
				Ω(fakeBBS.DesiredLRPsByDomainCallCount()).Should(Equal(1))
				Ω(fakeBBS.DesiredLRPsByDomainArgsForCall(0)).Should(Equal("domain-1"))
			})

			It("responds with 200 Status OK", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusOK))
			})

			It("returns a list of desired lrp responses", func() {
				response := []receptor.DesiredLRPResponse{}
				err := json.Unmarshal(responseRecorder.Body.Bytes(), &response)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(response).Should(HaveLen(2))
				Ω(response[0].ProcessGuid).Should(Equal("process-guid-0"))
				Ω(response[1].ProcessGuid).Should(Equal("process-guid-1"))
			})
		})

		Context("when the BBS returns no lrps", func() {
			BeforeEach(func() {
				fakeBBS.DesiredLRPsByDomainReturns([]models.DesiredLRP{}, nil)
			})

			It("call the BBS to retrieve the desired LRP", func() {
				Ω(fakeBBS.DesiredLRPsByDomainCallCount()).Should(Equal(1))
				Ω(fakeBBS.DesiredLRPsByDomainArgsForCall(0)).Should(Equal("domain-1"))
			})

			It("responds with 200 Status OK", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusOK))
			})

			It("returns an empty list", func() {
				Ω(responseRecorder.Body.String()).Should(Equal("[]"))
			})
		})

		Context("when the :domain is blank", func() {
			BeforeEach(func() {
				req.Form = url.Values{}
			})

			It("should not call the BBS to retrieve the desired LRP", func() {
				Ω(fakeBBS.DesiredLRPsByDomainCallCount()).Should(Equal(0))
			})

			It("responds with 400 BAD REQUEST", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusBadRequest))
			})

			It("responds with a relevant error message", func() {
				expectedBody, _ := json.Marshal(receptor.Error{
					Type:    receptor.InvalidRequest,
					Message: "domain missing from request",
				})

				Ω(responseRecorder.Body.String()).Should(Equal(string(expectedBody)))
			})
		})

		Context("when reading from the BBS fails", func() {
			BeforeEach(func() {
				fakeBBS.DesiredLRPsByDomainReturns([]models.DesiredLRP{}, errors.New("Something went wrong"))
			})

			It("responds with an error", func() {
				Ω(responseRecorder.Code).Should(Equal(http.StatusInternalServerError))
			})

			It("responds with a relevant error message", func() {
				expectedBody, _ := json.Marshal(receptor.Error{
					Type:    receptor.UnknownError,
					Message: "Something went wrong",
				})

				Ω(responseRecorder.Body.String()).Should(Equal(string(expectedBody)))
			})
		})
	})
})
