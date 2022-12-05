package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/hashicorp/go-memdb"
	"log"
	"net/http"
	"os"
	"time"
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Types ///////////////////////////////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type RequestError struct {
	StatusCode int
	Err        error
}

func (r *RequestError) Error() string {
	return r.Err.Error()
}

type pendingMeeting struct {
	eventId  string
	pageSlug string
	editHash string
}

type acceptMeetingRequest struct {
	Url string `json:"url"`
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Public //////////////////////////////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var DB *memdb.MemDB

// InitIMDB - leveraging an in memory datastore to keep the demo simple
func InitIMDB() {
	// Create the DB schema
	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"pendingMeetings": {
				Name: "pendingMeetings",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "eventId"},
					},
					"editHash": {
						Name:    "editHash",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "editHash"},
					},
				},
			},
		},
	}

	db, err := memdb.NewMemDB(schema)

	if err != nil {
		panic(err)
	}

	DB = db
}

// SavePendingMeeting - create the pending meeting and insert it into the IMDB
func SavePendingMeeting(eventId string, pageSlug string, editHash string) {
	p := pendingMeeting{
		eventId:  eventId,
		pageSlug: pageSlug,
		editHash: editHash,
	}

	log.Println("Inserted pending meeting with eventId:", eventId)
	insert(p)
}

// GetAndAcceptPendingMeeting - lookup the pending meeting using the event ID passed from the FE, accept the meeting since the payment was successful
func GetAndAcceptPendingMeeting(eventId string) {
	if meeting, found := getPendingMeeting(eventId); found {
		if err := acceptMeeting(meeting); err != nil {
			panic(err)
		} else {
			remove(meeting)
		}
	} else {
		log.Println("Meeting not found, unable to accept")
	}
}

// GetAndDeletePendingMeeting - lookup the pending meeting using the event ID passed from the FE, remove the meeting since the payment was unsuccessful
func GetAndDeletePendingMeeting(eventId string) {
	if meeting, found := getPendingMeeting(eventId); found {
		remove(meeting)
		if err := deleteFromNylas(meeting); err != nil {
			log.Println(err)
		}
	} else {
		log.Println("Meeting not found, unable to delete")
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Private /////////////////////////////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// buildAcceptUrl - build the URL that is passed to the accept utility for Node Puppeteer to accept the meeting for the organizer.  Note - not accounting for EU scheduler pages here.
func buildAcceptUrl(p pendingMeeting) string {
	return "https://schedule.nylas.com/" + p.pageSlug + "/confirm/" + p.editHash
}

// deleteFromNylas - call the Nylas API to cancel the initial meeting booked by the scheduler in the case where the payment was unsuccessful
func deleteFromNylas(p pendingMeeting) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", "api.nylas.com/"+p.eventId, nil)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+os.Getenv("NYLAS_ACCESS_TOKEN"))
	res, httpErr := http.DefaultClient.Do(req)

	if httpErr != nil {
		return &RequestError{StatusCode: 500, Err: httpErr}
	}

	if res.StatusCode != 200 {
		return &RequestError{StatusCode: res.StatusCode, Err: errors.New(res.Status)}
	}

	return nil
}

// getPendingMeeting - fetch a pending meeting from the IMDB using the event ID
func getPendingMeeting(eventId string) (pendingMeeting, bool) {
	var results []pendingMeeting

	txn := DB.Txn(true)
	defer txn.Abort()

	it, getErr := txn.Get("pendingMeetings", "id", eventId)
	if getErr != nil {
		panic(getErr)
	}

	for obj := it.Next(); obj != nil; obj = it.Next() {
		o := obj.(pendingMeeting)
		results = append(results, o)
	}

	if len(results) > 1 {
		panic("getPendingMeeting: found more than one matching event")
	} else if len(results) == 1 {
		return results[0], true
	}

	return pendingMeeting{}, false
}

// insert a pending meeting into the IMDB
func insert(p pendingMeeting) {
	txn := DB.Txn(true)
	defer txn.Abort()
	if err := txn.Insert("pendingMeetings", p); err != nil {
		panic(err)
	}
	txn.Commit()
}

// remove a pending meeting from the IMDB
func remove(p pendingMeeting) {
	txn := DB.Txn(true)
	defer txn.Abort()
	if err := txn.Delete("pendingMeetings", p); err != nil {
		panic(err)
	}
	txn.Commit()
}

// acceptMeeting by calling node server (see https://github.com/nickbair-nylas/acceptUtility)
func acceptMeeting(p pendingMeeting) error {
	payload, marshalErr := json.Marshal(acceptMeetingRequest{Url: buildAcceptUrl(p)})

	if marshalErr != nil {
		return &RequestError{
			StatusCode: 400,
			Err:        marshalErr,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost:3000/accept", bytes.NewBuffer(payload))
	req.Header.Add("Content-Type", "application/json")
	res, httpErr := http.DefaultClient.Do(req)

	if httpErr != nil {
		return &RequestError{StatusCode: 500, Err: httpErr}
	}

	if res.StatusCode != 200 {
		return &RequestError{StatusCode: res.StatusCode, Err: errors.New(res.Status)}
	}

	return nil
}
