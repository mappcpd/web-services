package schema

import (
	"github.com/graphql-go/graphql"

	"github.com/mappcpd/web-services/internal/activities"
)

// activity maps activities.activity
//type activity activities.Activity
//type activity struct {
//	ID           int    `json:"id" bson:"id"`
//	Code         string `json:"code" bson:"code"`
//	Name         string `json:"name" bson:"name"`
//	Description  string `json:"description" bson:"description"`
//	CategoryID   int    `json:"categoryId" bson:"categoryId"`
//	CategoryName string `json:"categoryName" bson:"categoryName"`
//	UnitID       int    `json:"unitId" bson:"unitId"`
//	UnitName     string
//	CreditPerUnit float32 `json:"creditPerUnit" bson:"creditPerUnit"`
//	//Credit       activities.ActivityCredit `json:"credit" bson:"credit"`
//}

// activityType is a local version of activities.ActivityType, to remove to the sql.NullInt64
type activityType struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// activitiesData returns a list of activity types
func activitiesData() ([]activities.Activity, error) {

	return activities.Activities()

	//var xla []activity
	//
	//xa, err := activities.Activities()
	//if err != nil {
	//	return nil, err
	//}
	//
	//// map to local type
	//for _, a := range xa {
	//	at := activity{}
	//	at.ID = a.ID
	//	at.Code = a.Code
	//	at.Name = a.Name
	//	at.Description = a.Description
	//	at.CategoryID = a.CategoryID
	//	at.CategoryName = a.CategoryName
	//	at.UnitID = a.UnitID
	//	at.UnitName = a.UnitName
	//	xla = append(xla, at)
	//}
	//
	//return xla,
}

// activityTypesData returns sub types for an activity
func activityTypesData(activityID int) ([]activities.ActivityType, error) {
	return activities.ActivityTypesByActivity(activityID)
}

// activityIDByActivityTypeID returns the activity id for an activity type id
func activityIDByActivityTypeID(activityTypeID int) (int, error) {

	a, err := activities.ActivityByActivityTypeID(activityTypeID)

	return a.ID, err
}

// activitiesQueryField resolves queries for activities (activity types)
var activitiesQueryField = &graphql.Field{
	Description: "Fetches a list of activity types.",
	Type:        graphql.NewList(activityQueryObject),
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		return activitiesData()
	},
}

// activityQueryObject defines the fields (properties) of an activity
var activityQueryObject = graphql.NewObject(graphql.ObjectConfig{
	Name: "activity",
	Description: "Activity describes a group of related activity types. This is the entity that includes the credit " +
		"value and caps for the activity (types) contained within.",
	Fields: graphql.Fields{
		"id": &graphql.Field{
			Type:        graphql.Int,
			Description: "The id of the activity",
		},
		"code": &graphql.Field{
			Type:        graphql.String,
			Description: "The code representing the activity",
		},
		"name": &graphql.Field{
			Type:        graphql.String,
			Description: "The name of the activity",
		},
		"description": &graphql.Field{
			Type:        graphql.String,
			Description: "A description of the activity",
		},
		"categoryId": &graphql.Field{
			Type:        graphql.Int,
			Description: "ID of the category to which the activity belongs",
		},
		"categoryName": &graphql.Field{
			Type:        graphql.String,
			Description: "Name of the category to which the activity belongs",
		},
		"unitId": &graphql.Field{
			Type:        graphql.Int,
			Description: "ID of the unit record used to measure the activity",
		},
		"unitName": &graphql.Field{
			Type:        graphql.String,
			Description: "Name of the unit used to measure the activity",
		},
		"creditPerUnit": &graphql.Field{
			Type:        graphql.Float,
			Description: "CPD credit per per unit of activity",
		},
		"types": activityTypesQueryField,
	},
})

// activityTypesQueryField resolves queries for activity types
var activityTypesQueryField = &graphql.Field{
	Description: "Fetches a list of activity types.",
	Type:        graphql.NewList(activityTypeQueryObject),
	Resolve: func(p graphql.ResolveParams) (interface{}, error) {

		// get the activity id from the parent (activity) object
		// note .Source is interface{} which can assert to activity
		activityID := p.Source.(activities.Activity).ID
		types, err := activityTypesData(activityID)
		if err != nil {
			return nil, nil
		}

		// Deal with sql.NullInt64 type from ce_activity.ce_activity_type_id
		var xat []activityType
		for _, v := range types {
			at := activityType{}
			if v.ID.Valid {
				at.ID = int(v.ID.Int64)
				at.Name = v.Name
				xat = append(xat, at)
			}
		}

		return xat, nil
	},
}

// activityTypeQueryObject defines the fields (properties) of an activity sub-type
var activityTypeQueryObject = graphql.NewObject(graphql.ObjectConfig{
	Name:        "activityType",
	Description: "Activity sub-types or examples.",
	Fields: graphql.Fields{
		"id": &graphql.Field{
			Type:        graphql.Int,
			Description: "The id of the activity sub-type, required when adding member activities",
		},
		"name": &graphql.Field{
			Type:        graphql.String,
			Description: "The name of the activity sub-type",
		},
	},
})
