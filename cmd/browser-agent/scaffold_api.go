// scaffold_api.go — Types and validation for POST /api/scaffold.

package main

// scaffoldRequest is the JSON body for POST /api/scaffold.
type scaffoldRequest struct {
	Description  string `json:"description"`
	Audience     string `json:"audience"`
	FirstFeature string `json:"first_feature"`
	Name         string `json:"name"`
}

// scaffoldResponse is the JSON response for POST /api/scaffold.
type scaffoldResponse struct {
	Status  string `json:"status"`
	Channel string `json:"channel"`
}

// validAudiences is the set of allowed audience values.
var validAudiences = map[string]bool{
	"just_me": true,
	"my_team": true,
	"public":  true,
}
