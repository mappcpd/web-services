package handlers

import (
	"fmt"
	"strconv"
	"time"

	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/imdario/mergo"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/mappcpd/web-services/cmd/webd/rest/router/handlers/responder"
	"github.com/mappcpd/web-services/cmd/webd/rest/router/middleware"
	"github.com/mappcpd/web-services/internal/activities"
	"github.com/mappcpd/web-services/internal/attachments"
	"github.com/mappcpd/web-services/internal/fileset"
	"github.com/mappcpd/web-services/internal/member/activity"
	"github.com/mappcpd/web-services/internal/platform/datastore"
	"github.com/mappcpd/web-services/internal/platform/s3"
)

// Activities fetches list of activity types
func Activities(w http.ResponseWriter, _ *http.Request) {

	p := responder.New(middleware.UserAuthToken.Token)

	al, err := activities.Activities()
	if err != nil {
		p.Message = responder.Message{http.StatusInternalServerError, "failed", err.Error()}
		p.Send(w)
		return
	}

	// All good
	p.Message = responder.Message{http.StatusOK, "success", "Data retrieved from " + datastore.MySQL.Source}
	p.Data = al
	m := make(map[string]interface{})
	m["count"] = len(al)
	m["description"] = "This is a list of Activity types for creating lists etc. The typeId is required for creating new Activity records"
	p.Meta = m
	p.Send(w)
}

// ActivitiesID fetches a single activity type by ID
func ActivitiesID(w http.ResponseWriter, r *http.Request) {

	p := responder.New(middleware.UserAuthToken.Token)

	// Request - convert id from string to int type
	v := mux.Vars(r)
	id, err := strconv.Atoi(v["id"])
	if err != nil {
		p.Message = responder.Message{http.StatusBadRequest, "failed", err.Error()}
	}

	a, err := activities.ActivityByID(id)
	if err != nil {
		p.Message = responder.Message{http.StatusInternalServerError, "failed", err.Error()}
		p.Send(w)
		return
	}

	// All good
	p.Message = responder.Message{http.StatusOK, "success", "Data retrieved from " + datastore.MySQL.Source}
	p.Data = a
	m := make(map[string]interface{})
	m["description"] = "The typeId must included when creating new Activity records"
	p.Meta = m
	p.Send(w)
}

// MembersActivitiesID fetches a single activity record by id
func MembersActivitiesID(w http.ResponseWriter, r *http.Request) {

	p := responder.New(middleware.UserAuthToken.Token)

	// Request - convert id from string to int type
	v := mux.Vars(r)
	id, err := strconv.Atoi(v["id"])
	if err != nil {
		p.Message = responder.Message{http.StatusBadRequest, "failed", err.Error()}
	}

	// Response
	a, err := activity.MemberActivityByID(int(id))
	switch {
	case err == sql.ErrNoRows:
		p.Message = responder.Message{http.StatusNotFound, "failed", err.Error()}
		p.Send(w)
		return
	case err != nil:
		p.Message = responder.Message{http.StatusInternalServerError, "failed", err.Error()}
		p.Send(w)
		return
	}

	// Authorization - need  owner of the record
	if middleware.UserAuthToken.Claims.ID != a.MemberID {
		p.Message = responder.Message{http.StatusUnauthorized, "failed", "Token does not belong to the owner of resource"}
		p.Send(w)
		return
	}

	// All good
	p.Message = responder.Message{http.StatusOK, "success", "Data retrieved from " + datastore.MySQL.Source}
	p.Data = a
	p.Send(w)
}

// MembersActivitiesAdd adds a new activity for the logged in member
func MembersActivitiesAdd(w http.ResponseWriter, r *http.Request) {

	p := responder.New(middleware.UserAuthToken.Token)

	// Decode JSON body into ActivityAttachment value
	a := activity.MemberActivityInput{}
	a.MemberID = middleware.UserAuthToken.Claims.ID
	err := json.NewDecoder(r.Body).Decode(&a)
	if err != nil {
		msg := "Error decoding JSON: " + err.Error() + ". Check the format of request body."
		p.Message = responder.Message{http.StatusBadRequest, "failure", msg}
		p.Send(w)
		return
	}

	aid, err := activity.AddMemberActivity(a)
	if err != nil {
		p.Message = responder.Message{http.StatusInternalServerError, "failure", err.Error()}
		p.Send(w)
		return
	}

	// Fetch the new record for return
	ar, err := activity.MemberActivityByID(int(aid))
	if err != nil {
		msg := "Could not fetch the new record"
		p.Message = responder.Message{http.StatusInternalServerError, "failure", msg + " " + err.Error()}
		p.Send(w)
		return
	}

	msg := fmt.Sprintf("Added a new activity (id: %v) for member (id: %v)", aid, middleware.UserAuthToken.Claims.ID)
	p.Message = responder.Message{http.StatusCreated, "success", msg}
	p.Data = ar
	p.Send(w)
}

// MembersActivitiesUpdate updates an existing activity for the logged in member.
// First we fetch the existing record into an Activity, and then replace the update fields with
// new values - this will be validated in the same way as a new activity and can also
// update one to many fields.
func MembersActivitiesUpdate(w http.ResponseWriter, r *http.Request) {

	p := responder.New(middleware.UserAuthToken.Token)

	// Get activity id from path... and make it an int
	v := mux.Vars(r)
	id, err := strconv.Atoi(v["id"])
	if err != nil {
		p.Message = responder.Message{http.StatusBadRequest, "failed", err.Error()}
	}

	// Fetch the original activity record
	a, err := activity.MemberActivityByID(int(id))
	switch {
	case err == sql.ErrNoRows:
		p.Message = responder.Message{http.StatusNotFound, "failed", err.Error()}
		p.Send(w)
		return
	case err != nil:
		p.Message = responder.Message{http.StatusInternalServerError, "failed", err.Error()}
		p.Send(w)
		return
	}

	// Authorization - need  owner of the record
	if middleware.UserAuthToken.Claims.ID != a.MemberID {
		p.Message = responder.Message{http.StatusUnauthorized, "failed", "Token does not belong to the owner of resource"}
		p.Send(w)
		return
	}

	// Original activity - from above we have a MemberActivity but need a subset of this - ie MemberActivityInput
	oa := activity.MemberActivityInput{
		ID:          a.ID,
		MemberID:    a.MemberID,
		ActivityID:  a.Activity.ID,
		Evidence:    0,
		Date:        a.Date,
		Quantity:    a.CreditData.Quantity,
		UnitCredit:  a.CreditData.UnitCredit,
		Description: a.Description,
	}

	// new activity - ie, updated version posted in JSON body
	na := activity.MemberActivityInput{}
	err = json.NewDecoder(r.Body).Decode(&na)
	if err != nil {
		msg := "Error decoding JSON: " + err.Error() + ". Check the format of request body."
		p.Message = responder.Message{http.StatusBadRequest, "failure", msg}
		p.Send(w)
		return
	}

	// Merge the original into the new record to fill in any blanks. The merge package
	// will only overwrite 'zero' values, so the updates are kept, and the nil values
	// back filled with the original values
	fmt.Println("Original:", oa)
	fmt.Println("New:", na)
	err = mergo.Merge(&na, oa)
	if err != nil {
		fmt.Println("Error merging activity fields: ", err)
	}
	fmt.Println("Original:", oa)
	fmt.Println("New:", na)

	// Update the activity record
	err = activity.UpdateMemberActivity(na)
	if err != nil {
		p.Message = responder.Message{http.StatusInternalServerError, "failure", err.Error()}
		p.Send(w)
		return
	}

	// updated record - fetch for response
	ur, err := activity.MemberActivityByID(int(id))
	if err != nil {
		msg := "Could not fetch the updated record"
		p.Message = responder.Message{http.StatusInternalServerError, "failure", msg + " " + err.Error()}
		p.Send(w)
		return
	}

	msg := fmt.Sprintf("Updated activity (id: %v) for member (id: %v)", id, middleware.UserAuthToken.Claims.ID)
	p.Message = responder.Message{http.StatusOK, "success", msg}
	p.Data = ur
	p.Send(w)
}

// MembersActivitiesRecurring fetches the member's recurring activities (if any) stored in MongoDB
func MembersActivitiesRecurring(w http.ResponseWriter, _ *http.Request) {

	p := responder.New(middleware.UserAuthToken.Token)

	ra, err := activity.MemberRecurring(middleware.UserAuthToken.Claims.ID)
	if err != nil {
		p.Message = responder.Message{http.StatusInternalServerError, "failed", "Failed to initialise a value of type MemberRecurring -" + err.Error()}
		p.Send(w)
		return
	}

	p.Message = responder.Message{http.StatusOK, "success", "Data retrieved from " + datastore.MongoDB.Source}
	p.Meta = map[string]int{"count": len(ra.Activities)}
	p.Data = ra
	p.Send(w)
}

// MembersActivitiesRecurringAdd adds a new recurring activity to the array in the Recurring doc that belongs to the member.
// Note that this function reads and writes only to MongoDB
func MembersActivitiesRecurringAdd(w http.ResponseWriter, r *http.Request) {

	p := responder.New(middleware.UserAuthToken.Token)

	// Get user id from token
	id := middleware.UserAuthToken.Claims.ID

	// Fetch the recurring activity doc for this user first
	ra, err := activity.MemberRecurring(id)
	if err != nil {
		msg := "MembersActivitiesRecurringAdd() Failed to initialise a value of type Recurring -" + err.Error()
		fmt.Println(msg)
		p.Message = responder.Message{http.StatusInternalServerError, "failed", msg}
		p.Send(w)
		return
	}
	ra.UpdatedAt = time.Now()

	// Decode the new activity from POST body...
	b := activity.RecurringActivity{}
	err = json.NewDecoder(r.Body).Decode(&b)
	if err != nil {
		msg := "MembersActivitiesRecurringAdd() failed to decode body -" + err.Error()
		fmt.Println(msg)
		p.Message = responder.Message{http.StatusInternalServerError, "failed", msg}
		p.Send(w)
		return
	}
	b.ID = bson.NewObjectId()
	b.CreatedAt = time.Now()
	b.UpdatedAt = time.Now()

	// Add the new recurring activity to the list...
	ra.Activities = append(ra.Activities, b)

	// ... and save
	err = ra.Save()
	if err != nil {
		p.Message = responder.Message{http.StatusInternalServerError, "failed", err.Error()}
		p.Send(w)
		return
	}

	p.Message = responder.Message{http.StatusOK, "success", "Data retrieved from " + datastore.MongoDB.Source}
	p.Meta = map[string]int{"count": len(ra.Activities)}
	p.Data = ra
	p.Send(w)
}

// MembersActivitiesRecurringRemove removes a recurring activity from the Recurring doc. Not it is not removing a
// doc in the collection, only one element from the array of recurring activities in the doc that belongs to the member
func MembersActivitiesRecurringRemove(w http.ResponseWriter, r *http.Request) {

	p := responder.New(middleware.UserAuthToken.Token)

	// Get user id from token
	id := middleware.UserAuthToken.Claims.ID

	// Fetch the recurring activity doc for this user first
	ra, err := activity.MemberRecurring(id)
	if err != nil {
		msg := "MembersActivitiesRecurringAdd() Failed to initialise a value of type Recurring -" + err.Error()
		fmt.Println(msg)
		p.Message = responder.Message{http.StatusInternalServerError, "failed", msg}
		p.Send(w)
		return
	}

	// Remove the recurring activity identified by the _id on url...
	_id := mux.Vars(r)["_id"]

	err = ra.RemoveActivity(_id)
	if err == mgo.ErrNotFound {
		msg := "No activity was found with id " + _id + " - it may have been already deleted"
		p.Message = responder.Message{http.StatusNotFound, "failure", msg + "... data retrieved from " + datastore.MongoDB.Source}

	} else if err != nil {
		msg := "An error occured - " + err.Error()
		p.Message = responder.Message{http.StatusInternalServerError, "failure", msg + "... data retrieved from " + datastore.MongoDB.Source}
	} else {
		p.Message = responder.Message{http.StatusOK, "success", "Data retrieved from " + datastore.MongoDB.Source}
	}

	p.Meta = map[string]int{"count": len(ra.Activities)}
	p.Data = ra
	p.Send(w)
}

// MembersActivitiesRecurringRecorder records a member activity based on a recurring activity.
// It creates a new member activity and then increments the next scheduled date for the recurring activity.
// If ?slip=1 is passed on the url then it will
func MembersActivitiesRecurringRecorder(w http.ResponseWriter, r *http.Request) {

	p := responder.Payload{}

	// Get the member's recurring activities. Strictly speaking we don't need the member id to do this
	// as we can select the document based on the recurring activity id. However, this ensures that the recurring
	// activity belongs to the member - however slim the chances of guessing an ObjectID!
	id := middleware.UserAuthToken.Claims.ID
	ra, err := activity.MemberRecurring(id)
	if err != nil {
		msg := "MembersActivitiesRecurringAdd() Failed to initialise a value of type Recurring -" + err.Error()
		fmt.Println(msg)
		p.Message = responder.Message{http.StatusInternalServerError, "failed", msg}
		p.Send(w)
		return
	}

	// Record (or skip) the target activity (_id on url), and increment the schedule
	_id := mux.Vars(r)["_id"]
	q := r.URL.Query()
	// ?skip=anything will do...
	if len(q["skip"]) > 0 {
		fmt.Println("Skip recurring activity...")
		err = ra.Skip(_id)
	} else {
		fmt.Println("Record recurring activity...")
		err = ra.Record(_id)
	}

	if err != nil {
		p.Message = responder.Message{http.StatusNotFound, "failed", "Could not record or skip recurring activity with id " + _id + " - " + err.Error()}
		p.Meta = map[string]int{"count": len(ra.Activities)}
		p.Data = ra
		p.Send(w)
		return
	}

	p.Message = responder.Message{http.StatusOK, "success", "Data retrieved from " + datastore.MongoDB.Source}
	p.Meta = map[string]int{"count": len(ra.Activities)}
	p.Data = ra
	p.Send(w)
}

// MembersActivitiesAttachmentRequest handles request for a signed URL to upload an attachment for a CPD activity
func MembersActivitiesAttachmentRequest(w http.ResponseWriter, r *http.Request) {

	p := responder.New(middleware.UserAuthToken.Token)

	upload := struct {
		SignedRequest  string `json:"signedRequest"`
		VolumeFilePath string `json:"volumeFilePath"`
		FileName       string `json:"fileName"`
		FileType       string `json:"fileType"`
	}{
		FileName: r.FormValue("filename"),
		FileType: r.FormValue("filetype"),
	}

	// Check we have required query params
	if upload.FileName == "" || upload.FileType == "" {
		msg := "Problems with query params, should have: ?filename=___&filetype=___"
		p.Message = responder.Message{http.StatusInternalServerError, "failed", msg}
		p.Send(w)
		return
	}

	// Check logged in member owns the activity record
	v := mux.Vars(r)
	id, err := strconv.Atoi(v["id"])
	if err != nil {
		msg := "Missing or malformed id in url path - " + err.Error()
		p.Message = responder.Message{http.StatusBadRequest, "failed", msg}
	}

	a, err := activity.MemberActivityByID(int(id))
	switch {
	case err == sql.ErrNoRows:
		msg := fmt.Sprintf("No activity found with id %d -", id) + err.Error()
		p.Message = responder.Message{http.StatusNotFound, "failed", msg}
		p.Send(w)
		return
	case err != nil:
		msg := "Database error - " + err.Error()
		p.Message = responder.Message{http.StatusInternalServerError, "failed", msg}
		p.Send(w)
		return
	}

	// Authorization - need  owner of the record
	if middleware.UserAuthToken.Claims.ID != a.MemberID {
		p.Message = responder.Message{http.StatusUnauthorized, "failed", "Token does not belong to the owner of resource"}
		p.Send(w)
		return
	}

	// Get current fileset for activity attachments
	fs, err := fileset.ActivityAttachment()
	if err != nil {
		msg := "Could not determine the storage information for activity attachments - " + err.Error()
		p.Message = responder.Message{http.StatusBadRequest, "failed", msg}
		p.Send(w)
		return
	}

	// Build FULL file path or 'key' in S3 parlance
	filePath := fs.Path + strconv.Itoa(id) + "/" + upload.FileName

	// Prepend the volume name to pass back to the client for subsequent file registration
	upload.VolumeFilePath = fs.Volume + filePath

	// get a signed request
	url, err := s3.PutRequest(filePath, fs.Volume)
	if err != nil {
		msg := "Error getting a signed request for upload " + err.Error()
		p.Message = responder.Message{http.StatusInternalServerError, "failed", msg}
		p.Send(w)
		return
	}
	upload.SignedRequest = url

	p.Message = responder.Message{http.StatusOK, "success", "Signed request in data.signedRequest."}
	p.Data = upload
	p.Send(w)
}

// MembersActivitiesAttachmentRegister registers an uploaded file in the database.
func MembersActivitiesAttachmentRegister(w http.ResponseWriter, r *http.Request) {

	p := responder.New(middleware.UserAuthToken.Token)

	a := attachments.New()
	// not required for this type of attachment but stick it on for good measure :)
	a.UserID = middleware.UserAuthToken.Claims.ID

	// Get the entity ID from URL path... This is admin so validate record exists but not ownership
	v := mux.Vars(r)
	id, err := strconv.Atoi(v["id"])
	if err != nil {
		msg := "Error getting id from url path - " + err.Error()
		p.Message = responder.Message{http.StatusBadRequest, "failed", msg}
		p.Data = a
		p.Send(w)
		return
	}
	activity, err := activity.MemberActivityByID(int(id))
	switch {
	case err == sql.ErrNoRows:
		msg := fmt.Sprintf("No activity found with id %d -", id) + err.Error()
		p.Message = responder.Message{http.StatusNotFound, "failed", msg}
		p.Data = a
		p.Send(w)
		return
	case err != nil:
		msg := "Database error - " + err.Error()
		p.Message = responder.Message{http.StatusInternalServerError, "failed", msg}
		p.Data = a
		p.Send(w)
		return
	}
	// CHECK OWNER!!
	if middleware.UserAuthToken.Claims.ID != activity.MemberID {
		p.Message = responder.Message{http.StatusUnauthorized, "failed", "Token does not belong to the owner of this resource"}
		p.Data = a
		p.Send(w)
		return
	}
	a.EntityID = int(id)

	// Decode post body fields: "cleanFilename" and "cloudyFilename" into Attachment
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		msg := "Could not decode json in request body - " + err.Error()
		p.Message = responder.Message{http.StatusBadRequest, "failed", msg}
		p.Data = a
		p.Send(w)
		return
	}

	// Get current fileset for activity attachments
	fs, err := fileset.ActivityAttachment()
	if err != nil {
		msg := "Could not determine the storage information for activity attachments - " + err.Error()
		p.Message = responder.Message{http.StatusInternalServerError, "failed", msg}
		p.Data = a
		p.Send(w)
		return
	}
	a.FileSet = fs

	// Register the attachment
	if err := a.Register(); err != nil {
		msg := "Error registering attachment - " + err.Error()
		p.Message = responder.Message{http.StatusInternalServerError, "failed", msg}
		p.Data = a
		p.Send(w)
		return
	}

	p.Message = responder.Message{http.StatusOK, "success", "Attachment registered"}
	p.Data = a
	p.Send(w)
}
