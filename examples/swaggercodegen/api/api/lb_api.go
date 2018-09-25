/*
 * Dummy Service Provider generated using 'swaggercodegen' that has two resources 'cdns' and 'lbs' which are terraform compliant
 *
 * This service provider allows the creation of fake 'cdns' and 'lbs' resources
 *
 * API version: 1.0.0
 * Contact: apiteam@serviceprovider.io
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */

package api

import (
	"fmt"
	"github.com/pborman/uuid"
	"log"
	"net/http"
	"strings"
	"time"
)

var defaultTimeToProcess int32 = 30 // 30 seconds
var lbsDB = map[string]*Lbv1{}

type status string

const (
	deployPending    status = "deploy_pending"
	deployInProgress status = "deploy_in_progress"
	deployFailed     status = "deploy_failed"
	deployed         status = "deployed"
	deletePending    status = "delete_pending"
	deleteInProgress status = "delete_in_progress"
	deleteFailed     status = "delete_failed"
	deleted          status = "deleted"
)

var deployPendingStatuses = []status{deployInProgress}
var deletePendingStatuses = []status{deleteInProgress}

func LBGetV1(w http.ResponseWriter, r *http.Request) {
	lb, err := retrieveLB(r)
	if err != nil {
		sendErrorResponse(http.StatusNotFound, err.Error(), w)
		return
	}
	sendResponse(http.StatusOK, w, lb)
}

func LBCreateV1(w http.ResponseWriter, r *http.Request) {
	lb := &Lbv1{}
	err := readRequest(r, lb)
	if err != nil {
		sendErrorResponse(http.StatusBadRequest, err.Error(), w)
		return
	}
	UpdateLBV1(lb, uuid.New(), lb.Name, lb.Backends, lb.SimulateFailure, lb.TimeToProcess, deployPending)

	go pretendResourceOperationIsProcessing(lb, deployPendingStatuses, deployed, deployFailed)

	sendResponse(http.StatusAccepted, w, lb)
}

func LBUpdateV1(w http.ResponseWriter, r *http.Request) {
	lb, err := retrieveLB(r)
	if err != nil {
		sendErrorResponse(http.StatusNotFound, err.Error(), w)
		return
	}
	newLB := &Lbv1{}
	err = readRequest(r, newLB)
	if err != nil {
		sendErrorResponse(http.StatusBadRequest, err.Error(), w)
		return
	}
	UpdateLBV1(lb, lb.Id, newLB.Name, newLB.Backends, newLB.SimulateFailure, newLB.TimeToProcess, deployPending)

	go pretendResourceOperationIsProcessing(lb, deployPendingStatuses, deployed, deployFailed)

	sendResponse(http.StatusAccepted, w, lbsDB[lb.Id])
}

func UpdateLBV1(lb *Lbv1, id string, name string, backends []string, simulateFailure bool, timeToProcess int32, newStatus status) {
	lb.Id = id
	lb.Name = name
	lb.Backends = backends
	lb.SimulateFailure = simulateFailure
	lb.TimeToProcess = timeToProcess
	updateLBStatus(lb, newStatus)
	lbsDB[lb.Id] = lb
}

func LBDeleteV1(w http.ResponseWriter, r *http.Request) {
	lb, err := retrieveLB(r)
	if err != nil {
		sendErrorResponse(http.StatusNotFound, err.Error(), w)
		return
	}
	updateLBStatus(lb, deletePending)

	go pretendResourceOperationIsProcessing(lb, deletePendingStatuses, deleted, deleteFailed)

	sendResponse(http.StatusAccepted, w, nil)
}

func pretendResourceOperationIsProcessing(lb *Lbv1, pendingStatues []status, completed status, failureStatus status) {
	var timeToProcess = defaultTimeToProcess
	// Override default wait time if it is configured in the lb
	if lb.TimeToProcess > 0 {
		timeToProcess = lb.TimeToProcess
	}
	var finalStatus status
	var inProgressStatuses []status
	if lb.SimulateFailure {
		log.Printf("Simulating failure for '%s'", lb.Id)
		inProgressStatuses = []status{failureStatus}
		finalStatus = failureStatus
	} else {
		inProgressStatuses = pendingStatues
		finalStatus = completed
	}
	waitTimePerPendingStatus := timeToProcess / int32(len(inProgressStatuses)+1)
	for _, newStatus := range inProgressStatuses {
		sleepAndUpdateLB(lb, newStatus, waitTimePerPendingStatus)
	}
	// This is the case of delete operation; where there is no completed status as at point the resource should be destroyed completely
	if completed == deleted {
		sleepAndDestroyLB(lb, waitTimePerPendingStatus)
	} else {
		sleepAndUpdateLB(lb, finalStatus, waitTimePerPendingStatus)
	}
}

func sleepAndUpdateLB(lb *Lbv1, newStatus status, waitTime int32) {
	timeToProcessPerStatusDuration := time.Duration(waitTime) * time.Second
	log.Printf("Precessing resource [%s] [%s => %s] - timeToProcess = %ds", lb.Id, lb.Status, newStatus, waitTime)
	time.Sleep(timeToProcessPerStatusDuration)
	updateLBStatus(lb, newStatus)
}

func sleepAndDestroyLB(lb *Lbv1, waitTime int32) {
	timeToProcessPerStatusDuration := time.Duration(waitTime) * time.Second
	log.Printf("Destroying resource [%s] [%s] - timeToProcess = %ds", lb.Id, lb.Status, waitTime)
	time.Sleep(timeToProcessPerStatusDuration)
	delete(lbsDB, lb.Id)
	log.Printf("resource [%s] destroyed", lb.Id)
}

func updateLBStatus(lb *Lbv1, newStatus status) {
	oldStatus := lb.Status
	lb.Status = string(newStatus)
	log.Printf("LB [%s] status updated '%s' => '%s'", lb.Id, oldStatus, newStatus)
}

func retrieveLB(r *http.Request) (*Lbv1, error) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/lbs/")
	if id == "" {
		return nil, fmt.Errorf("lb id path param not provided")
	}
	lb, exists := lbsDB[id]
	if lb == nil || !exists {
		return nil, fmt.Errorf("lb id '%s' not found", id)
	}
	return lb, nil
}
