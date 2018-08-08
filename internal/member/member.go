package member

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cardiacsociety/web-services/internal/cpd"
	"github.com/cardiacsociety/web-services/internal/date"
	"github.com/cardiacsociety/web-services/internal/platform/datastore"
	"github.com/pkg/errors"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"strconv"
	)

// Note trying to scan NULL db values into strings throws an error. This is discussed here:
// https://github.com/go-sql-driver/mysql/issues/34
// Using []byte is a workaround but then need to convert back to strings. So I've used
// COALESCE() in any SQL where a NULL value is possible... it is a problem with the db
// so might as well make the db deal with it :)

// this file contains the Member "model" -  a struct that maps to the JSON representation
// of the member record represented as a document, and can be unpacked to be mapped to the
// relational model ofr a member

// Member defines struct for member record
type Member struct {
	OID       bson.ObjectId `json:"_id,omitempty" bson:"_id,omitempty"`
	ID        int           `json:"id" bson:"id"`
	CreatedAt time.Time     `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt" bson:"updatedAt"`

	// Active refers to the members status in relation to the organisation, ie ms_m_status.ms_status_id = 1 (MySQL)
	// In this model this really belongs in the memberships, however is here from simplicity.
	Active         bool            `json:"active" bson:"active"`
	Title          string          `json:"title" bson:"title"`
	FirstName      string          `json:"firstName" bson:"firstName"`
	MiddleNames    []string        `json:"middleNames" bson:"middleNames"`
	LastName       string          `json:"lastName" bson:"lastName"`
	PostNominal    string          `json:"postNominal" bson:"postNominal"`
	Gender         string          `json:"gender" bson:"gender"`
	DateOfBirth    string          `json:"dateOfBirth" bson:"dateOfBirth"`
	Memberships    []Membership    `json:"memberships" bson:"memberships"`
	Contact        Contact         `json:"contact" bson:"contact"`
	Qualifications []Qualification `json:"qualifications" bson:"qualifications"`
	Accreditations []Accreditation `json:"accreditations" bson:"accreditations"`
	Positions      []Position      `json:"positions" bson:"positions"`
	Specialities   []Speciality    `json:"specialities" bson:"specialities"`

	// omitempty to exclude this from sync
	RecurringActivities []cpd.RecurringActivity `json:"recurringActivities,omitempty" bson:"recurringActivities,omitempty"`
}

type Members []Member

// Contact struct holds all Contact information for a member
type Contact struct {
	EmailPrimary   string     `json:"emailPrimary" bson:"emailPrimary"`
	EmailSecondary string     `json:"emailSecondary" bson:"emailSecondary"`
	Mobile         string     `json:"mobile" bson:"mobile"`
	Locations      []Location `json:"locations" bson:"locations"`

	// Flags that indicate members consent to appear in the directory, and to have Contact details shared in directory
	Directory bool `json:"directory" bson:"directory"`
	Consent   bool `json:"consent" bson:"consent"`
}

// Location defines a Contact place or Contact 'card'
type Location struct {
	Preference  int      `json:"preference,omitempty" bson:"preference"`
	Description string   `json:"type,omitempty" bson:"type"`
	Address     []string `json:"address,omitempty" bson:"address"`
	City        string   `json:"city,omitempty" bson:"city"`
	State       string   `json:"state,omitempty" bson:"state"`
	Postcode    string   `json:"postcode,omitempty" json:"postcode"`
	Country     string   `json:"country,omitempty" bson:"country"`
	Phone       string   `json:"phone,omitempty" bson:"phone"`
	Fax         string   `json:"fax,omitempty" bson:"fax"`
	Email       string   `json:"email,omitempty" bson:"email"`
	URL         string   `json:"url,omitempty" bson:"url"`
}

// Membership holds all of the details for membership to an organisation
type Membership struct {
	OrgID         string             `json:"orgId" bson:"orgId"`
	OrgCode       string             `json:"orgCode" bson:"orgCode"`
	OrgName       string             `json:"orgName" bson:"orgName"`
	Title         string             `json:"title" bson:"title"`
	TitleHistory  []MembershipTitle  `json:"titleHistory" bson:"titleHistory"`
	Status        string             `json:"status" bson:"status"`
	StatusHistory []MembershipStatus `json:"statusHistory" bson:"statusHistory"`
}

// MembershipTitle refers to the standing, rank or type of membership within an organisation
type MembershipTitle struct {
	Date        string `json:"date" bson:"date"`
	Name        string `json:"title" bson:"title"`
	Description string `json:"description,omitempty" bson:"description"`
	Comment     string `json:"comment,omitempty" bson:"comment"`
}

// MembershipStatus refers to the membership status - eg active, lapsed, retired etc, of a membership within an organisation
type MembershipStatus struct {
	Date        string `json:"date" bson:"date"`
	Name        string `json:"status" bson:"status"`
	Description string `json:"description,omitempty" bson:"description"`
	Comment     string `json:"comment,omitempty" bson:"comment"`
}

// Qualification is a formal qualification such as a degree, Masters, PHD etc
type Qualification struct {
	Code        string `json:"code" bson:"code"`
	Name        string `json:"name" bson:"name"`
	Description string `json:"description,omitempty" bson:"description"`
	Year        int    `json:"year,omitempty" bson:"year"`
}

// Accreditation is an industry-approval for a particular practice or process
type Accreditation struct {
	Code        string `json:"code" bson:"code"`
	Name        string `json:"name" bson:"name"`
	Description string `json:"description,omitempty" bson:"description"`
	Start       string `json:"start,omitempty" bson:"start"`
	End         string `json:"end,omitempty" bson:"end"`
}

// Position is an appointment to a board, council or similar
type Position struct {
	OrgCode     string `json:"orgCode" bson:"orgCode"`
	OrgName     string `json:"orgName" bson:"orgName"`
	Code        string `json:"code,omitempty" bson:"code"`
	Name        string `json:"name" bson:"name"`
	Description string `json:"description,omitempty" bson:"description"`
	Start       string `json:"start,omitempty" bson:"start"`
	End         string `json:"end,omitempty" bson:"end"`
}

// Speciality are particular areas of professional expertise or interest
type Speciality struct {
	Name        string `json:"name" bson:"name"`
	Description string `json:"description,omitempty" bson:"description"`
	Start       string `json:"start,omitempty" bson:"start"`
}

// SetHonorific sets the title (Mr, Prof, Dr) and Post nominal, if any
func (m *Member) SetHonorific(ds datastore.Datastore) error {

	query := Queries["select-member-honorific"]
	err := ds.MySQL.Session.QueryRow(query, m.ID).Scan(&m.Title)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "SetHonorific query error")
	}

	return nil
}

// SetContactLocations populates the Contact.Locations []Location
func (m *Member) SetContactLocations(ds datastore.Datastore) error {

	query := Queries["select-member-contact-locations"]
	rows, err := ds.MySQL.Session.Query(query, m.ID)
	if err == sql.ErrNoRows {
		return nil

	}
	if err != nil {
		return errors.Wrap(err, "SetContactLocations query error")
	}
	defer rows.Close()

	for rows.Next() {

		var l Location
		var address string

		err := rows.Scan(
			&l.Description,
			&address,
			&l.City,
			&l.State,
			&l.Postcode,
			&l.Country,
			&l.Phone,
			&l.Fax,
			&l.Email,
			&l.URL,
			&l.Preference,
		)
		if err != nil {
			return errors.Wrap(err, "SetContactLocations scan")
		}

		// split address string into array of lines
		xa := strings.Split(address, "\n")
		for _, a := range xa {
			if len(a) > 0 {
				l.Address = append(l.Address, a)
			}
		}

		m.Contact.Locations = append(m.Contact.Locations, l)
	}

	return nil
}

// GetMemberships populates the Memberships field with one or more Membership values - hard coded to CSANZ for now
func (m *Member) SetMemberships() error {

	// TODO: Add a field called CustomData for any JSON specific to the Membership
	csanz := Membership{
		OrgID:   "csanz",
		OrgCode: "CSANZ",
		OrgName: "Cardiac Society of Australia and New Zealand",
	}

	m.Memberships = append(m.Memberships, csanz)

	return nil
}

// SetMembershipTitle populates the MembershipTitle field for a particular Membership.
// It receives the Membership index (mi) which points to the relevant item in []Membership
func (m *Member) SetMembershipTitle(ds datastore.Datastore, mi int) error {

	// For now we will just set the Member.MembershipTitle field to a string
	// with the name of the title. TitleHistory contains all the details
	// Including the current title so storing them at Member.MembershipTitle is
	// somewhat redundant, and leaving the current title out of the
	// History seems silly as well, as it is part of the history.
	//t := MembershipTitle{}
	t := ""

	query := Queries["select-membership-title"]
	err := ds.MySQL.Session.QueryRow(query, m.ID).Scan(&t)
	if err == sql.ErrNoRows {
		// remove the default membership as there is no title
		m.Memberships = []Membership{}
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "SetMembershipTitle error")
	}

	m.Memberships[mi].Title = t
	return nil

}

// GetTitleHistory populates the Member.TitleHistory field for the Membership
// at index 'mi. Very similar to GetTitle except there may be more than one
// MembershipTitle so it uses []MembershipTitle
func (m *Member) SetMembershipTitleHistory(ds datastore.Datastore, mi int) error {

	query := Queries["select-membership-title-history"]
	rows, err := ds.MySQL.Session.Query(query, m.ID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "SetMembershipTitleHistory query error")
	}
	defer rows.Close()

	for rows.Next() {

		t := MembershipTitle{}
		err := rows.Scan(
			&t.Date,
			&t.Name,
			&t.Description,
			&t.Comment,
		)
		if err != nil {
			return errors.Wrap(err, "SetMembershipTitleHistory scan error")
		}

		m.Memberships[mi].TitleHistory = append(m.Memberships[mi].TitleHistory, t)
	}

	return nil
}

// SetMembershipStatus populates the MembershipStatus field for a particular Membership.
// It receives the Membership index (mi) which points to the relevant item in []Membership
func (m *Member) SetMembershipStatus(ds datastore.Datastore, mi int) error {

	s := ""
	query := Queries["select-membership-status"]
	err := ds.MySQL.Session.QueryRow(query, m.ID).Scan(&s)
	if err == sql.ErrNoRows {
		// remove the default membership as there is no status
		m.Memberships = []Membership{}
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "SetMembershipStatus error")
	}

	m.Memberships[mi].Status = s
	return nil
}

// SetMembershipStatusHistory populates the Member.StatusHistory field for the Membership at index mi.
func (m *Member) SetMembershipStatusHistory(ds datastore.Datastore, mi int) error {

	query := Queries["select-membership-status-history"]
	rows, err := ds.MySQL.Session.Query(query, m.ID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "SetMembershipStatusHistory query")
	}
	defer rows.Close()

	for rows.Next() {

		t := MembershipStatus{}
		err := rows.Scan(
			&t.Date,
			&t.Name,
			&t.Description,
			&t.Comment,
		)
		if err != nil {
			return errors.Wrap(err, "SetMembershipStatusHistory scan")
		}

		if len(m.Memberships) > 0 {
			m.Memberships[mi].StatusHistory = append(m.Memberships[mi].StatusHistory, t)
		}
	}

	return nil
}

// SetQualifications sets the qualifications
func (m *Member) SetQualifications(ds datastore.Datastore) error {

	query := Queries["select-member-qualifications"]
	rows, err := ds.MySQL.Session.Query(query, m.ID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "SetQualifications query error")
	}
	defer rows.Close()

	for rows.Next() {

		var q Qualification
		var year string

		err := rows.Scan(
			&q.Code,
			&q.Name,
			&q.Description,
			&year,
		)
		if err != nil {
			return errors.Wrap(err, "SetQualifications scan error")
		}

		if len(year) > 0 {
			q.Year, err = strconv.Atoi(year)
			if err != nil {
				return errors.Wrap(err, "SetQualifications could not convert year to integer")
			}
		}

		m.Qualifications = append(m.Qualifications, q)
	}

	return nil
}

// SetAccreditations adds member accreditations
func (m *Member) SetAccreditations(ds datastore.Datastore) error {

	query := Queries["select-member-accreditations"]
	rows, err := ds.MySQL.Session.Query(query, m.ID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "SetAccreditations query")
	}
	defer rows.Close()

	for rows.Next() {
		var a Accreditation
		err := rows.Scan(
			&a.Code,
			&a.Name,
			&a.Description,
			&a.Start,
			&a.End,
		)
		if err != nil {
			return errors.Wrap(err, "SetAccreditation scan")
		}

		m.Accreditations = append(m.Accreditations, a)
	}

	return nil
}

// SetPositions fetches the Positions held by a member and sets the corresponding fields
func (m *Member) SetPositions(ds datastore.Datastore) error {

	query := Queries["select-member-positions"]

	rows, err := ds.MySQL.Session.Query(query, m.ID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "SetPositions query error")
	}
	defer rows.Close()

	for rows.Next() {

		p := Position{}

		err := rows.Scan(
			&p.OrgCode,
			&p.OrgName,
			&p.Code,
			&p.Name,
			&p.Description,
			&p.Start,
			&p.End,
		)
		if err != nil {
			return errors.Wrap(err, "SetPositions scan error")
		}

		m.Positions = append(m.Positions, p)
	}

	return nil
}

// SetSpecialities fetches the specialities for a member and sets the corresponding fields
func (m *Member) SetSpecialities(ds datastore.Datastore) error {

	query := Queries["select-member-specialities"]
	rows, err := ds.MySQL.Session.Query(query, m.ID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "SetSpecialities query error")
	}
	defer rows.Close()

	for rows.Next() {

		s := Speciality{}

		err := rows.Scan(
			&s.Name,
			&s.Description,
			&s.Start,
		)
		if err != nil {
			return errors.Wrap(err, "SetSpecialities scan error")
		}

		m.Specialities = append(m.Specialities, s)
	}

	return nil
}

// SaveDocDB method upserts Member doc to MongoDB
func (m *Member) SaveDocDB(ds datastore.Datastore) error {

	selector := map[string]int{"id": m.ID}

	mc, err := ds.MongoDB.MembersCollection()
	if err != nil {
		return errors.Wrap(err, "SaveDocDB could not get member collection")
	}

	_, err = mc.Upsert(selector, &m)
	if err != nil {
		return errors.Wrap(err, "SaveDocDB upsert error")
	}

	return nil
}

// SyncUpdated synchronises a Member value to MongoDB based on the UpdatedAt field
func (m *Member) SyncUpdated(ds datastore.Datastore) error {

	xm, err := SearchDocDB(ds, bson.M{"id": m.ID})
	if err != nil && err != mgo.ErrNotFound {
		return errors.Wrap(err, "SyncUpdated Mongo query error")
	}

	if len(xm) > 1 {
		return errors.New(fmt.Sprintf("SyncUpdated found %v sync targets - should only be one!", len(xm)))
	}

	if err == mgo.ErrNotFound {
		return m.SaveDocDB(ds)
	}

	if len(xm) > 0 {
		if m.UpdatedAt.After(xm[0].UpdatedAt) {
			return m.SaveDocDB(ds)
		}
	}

	// do nothing
	return nil
}

// ByID returns a pointer to a populated Member value
func ByID(ds datastore.Datastore, id int) (*Member, error) {

	m := Member{ID: id}

	query := Queries["select-member"]

	var active int
	var createdAt string
	var updatedAt string
	var middleNames string

	err := ds.MySQL.Session.QueryRow(query, id).Scan(
		&active,
		&createdAt,
		&updatedAt,
		&m.FirstName,
		&middleNames,
		&m.LastName,
		&m.PostNominal,
		&m.Gender,
		&m.DateOfBirth,
		&m.Contact.EmailPrimary,
		&m.Contact.EmailSecondary,
		&m.Contact.Mobile,
		&m.Contact.Directory,
		&m.Contact.Consent,
	)

	if err == sql.ErrNoRows {
		return &m, errors.Wrap(err, "No member record with that id")
	}
	if err != nil {
		return &m, errors.Wrap(err, "SQL error")
	}

	// Note - this is soft-delete active NOT membership active
	if active == 1 {
		m.Active = true
	}

	m.CreatedAt, err = date.StringToTime(createdAt)
	if err != nil {
		return &m, errors.Wrap(err, "Error converting createdAt to Time")
	}
	m.UpdatedAt, err = date.StringToTime(updatedAt)
	if err != nil {
		return &m, errors.Wrap(err, "Error converting updatedAt to Time")
	}

	err = m.SetHonorific(ds)
	if err != nil {
		return &m, errors.Wrap(err, "SetHonorific")
	}

	if len(middleNames) > 0 {
		xmn := strings.Split(middleNames, " ")
		for _, mn := range xmn {
			m.MiddleNames = append(m.MiddleNames, mn)
		}
	}

	err = m.SetContactLocations(ds)
	if err != nil {
		return &m, errors.Wrap(err, "SetContactLocations")
	}

	err = m.SetMemberships()
	if err != nil {
		return &m, errors.Wrap(err, "SetMemberships")
	}
	for i := range m.Memberships {

		err = m.SetMembershipTitle(ds, i)
		if err != nil {
			return &m, errors.Wrap(err, "SetMembershipTitle")
		}

		err = m.SetMembershipTitleHistory(ds, i)
		if err != nil {
			return &m, errors.Wrap(err, "SetMembershipTitleHistory")
		}
	}

	for i := range m.Memberships {

		err = m.SetMembershipStatus(ds, i)
		if err != nil {
			return &m, errors.Wrap(err, "SetMembershipStatus")
		}

		err = m.SetMembershipStatusHistory(ds, i)
		if err != nil {
			return &m, errors.Wrap(err, "SetMembershipStatusHistory")
		}
	}

	err = m.SetQualifications(ds)
	if err != nil {
		return &m, errors.Wrap(err, "SetQualifications")
	}

	err = m.SetAccreditations(ds)
	if err != nil {
		return &m, errors.Wrap(err, "SetAccreditations")
	}

	err = m.SetPositions(ds)
	if err != nil {
		return &m, errors.Wrap(err, "SetPositions")
	}

	err = m.SetSpecialities(ds)
	if err != nil {
		return &m, errors.Wrap(err, "SetSpecialities")
	}

	return &m, nil
}

// SearchDocDB searches the Member collection using the specified query
func SearchDocDB(ds datastore.Datastore, query bson.M) ([]Member, error) {

	var xm []Member

	members, err := ds.MongoDB.MembersCollection()
	if err != nil {
		return nil, err
	}

	// .All(0 never returns ErrNotFound so need to check for results length = 0
	err = members.Find(query).All(&xm)
	if err != nil {
		return nil, err
	}
	if len(xm) == 0 {
		return nil, mgo.ErrNotFound
	}

	return xm, nil
}
