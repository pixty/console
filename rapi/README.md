# API brief comments

Copied from api.go:

```
	// The ping returns pong and URI of the ping, how we see it.
	a.ge.GET("/ping", a.h_GET_ping)

	// Create new secured session JSON {"login": "user", "password": "abc"} is exepected
	a.ge.POST("/sessions", a.h_POST_sessions)

	// Delete session by its sessionId
	a.ge.DELETE("/sessions/:sessId", a.h_DELETE_sessions_sessId)

	// Returns a composite object which contains list of persons(different faces) seen
	// from a time (or last seen) sorted in descending order. Every object in the list
	// is a Person JSON, which has references to list of faces of the person,
	// assigned profile (if it was) and matched profiles (the person seems to be
	// already associated). Profiles can be selected with meta data (fields) or
	// not.
	// Allowed paramas are:
	// - limit: number of records to be selected
	// - maxTime: the maximum time where the first person in the list last seen(all
	// 	other persons will have same or less time) - used for paging
	//
	// Example: curl https://api.pixty.io/cameras/12/timeline?limit=20&maxTime=12341234
	a.ge.GET("/cameras/:camId/timeline", a.h_GET_cameras_timeline)

	// Get an known image by its file name
	// Example: curl https://api.pixty.io/images/cm-1-504241500992.png
	a.ge.GET("/images/:imgName", a.h_GET_images_png_download)

	// Create new org - will be used by superadmin only
	a.ge.POST("/orgs", a.h_POST_orgs)

	// Gets all authenticated user's organizations JSON object
	a.ge.GET("/orgs", a.h_GET_orgs)

	// Gets organization JSON object
	a.ge.GET("/orgs/:orgId", a.h_GET_orgs_orgId)

	// Creates list of fields for specified organization. Every fields should
	// have display name and type (only 'text' is allowed) for now. Not for often use
	// when the field is deleted all profiles lost its values
	//
	// Example: curl -v -H "Content-Type: application/json" -X POST -d '[{"fieldName": "First Name", "fieldType": "text"}, { "fieldName": "Last Name", "fieldType": "text"}]' http://api.pixty.io/orgs/1/fields
	a.ge.POST("/orgs/:orgId/fields", a.h_POST_orgs_orgId_fields)

	// Get list of fields for the organization
	a.ge.GET("/orgs/:orgId/fields", a.h_GET_orgs_orgId_fields)

	// Gets field by its id
	a.ge.PUT("/orgs/:orgId/fields/:fldId", a.h_PUT_orgs_orgId_fields_fldId)

	// Delete an organization field - all data will be lost
	a.ge.DELETE("/orgs/:orgId/fields/:fldId", a.h_DELETE_orgs_orgId_fields_fldId)

	// returns list of user roles assignments
	a.ge.GET("/orgs/:orgId/userRoles", a.h_GET_orgs_orgId_userRoles)

	// Allows to assign user role
	a.ge.POST("/orgs/:orgId/userRoles", a.h_POST_orgs_orgId_userRoles)

	// Removes user role
	a.ge.DELETE("/orgs/:orgId/userRoles/:userId", a.h_DELETE_orgs_orgId_userRoles_userId)

	// Creates new user. The request accepts password optional field which allows
	// to set a new password due to creation. If it is not providede the password is empty.
	a.ge.POST("/users", a.h_POST_users)

	// Changes the user password. Only owner or superadmin can make the change.
	// Authenticated session is not affected
	a.ge.POST("/users/:userId/password", a.h_POST_users_userId_password)

	// Returns user info by the userId. Only owner and superadmin are authorized
	a.ge.GET("/users/:userId", a.h_GET_users_userId)

	// Returns user roles assigned through all orgs. Only owner and superadmin are authorized
	a.ge.GET("/users/:userId/userRoles", a.h_GET_users_userId_userRoles)

	// Creates a new profile. The call allows to provide some list of field values
	//
	// Example: curl -v -H "Content-Type: application/json" -X POST -d '{"AvatarUrl": "https://api/pixty.io/images/cm-1-1504241567000_731_353_950_572.png", "Attributes": [{"FieldId": 1, "Value": "Dmitry"}, {"FieldId": 2, "Value": "Spasibenko"}]}' http://api.pixty.io/profiles
	a.ge.POST("/profiles", a.h_POST_profiles)

	// Gets profile by its id. Only not empty fields will be returned(!)
	a.ge.GET("/profiles/:prfId", a.h_GET_profiles_prfId)

	// Gets profile persons by its id. Persons will not contain profile or matches references
	a.ge.GET("/profiles/:prfId/persons", a.h_GET_profiles_prfId_persons)

	// Updates profile AvatarUrl and list values. All fieds will be updated like
	// provided. It is not a PATCH, if a field is not set, it is considered as
	// removed. It is SNAPSHOT UPDATE
	a.ge.PUT("/profiles/:prfId", a.h_PUT_profiles_prfId)

	// Merges 2 profiles. It actually just re-assigns all persons with profileId=prf2Id
	// to prf1Id
	a.ge.POST("/profiles/:prf1Id/merge/:prf2Id", a.h_POST_profiles_merge)

	// Delete the profile
	a.ge.DELETE("/profiles/:prfId", a.h_DELETE_profiles_prfId)

	// Retrieves person by its id. The call can be light or include profiles and
	// pictures information. THe following query params are allowed:
	// - datails=true: includes information about the person pictures and profiles matched
	// - meta=true: includes fields in profiles
	a.ge.GET("/persons/:persId", a.h_GET_persons_persId)

	// Updates either avatar or profile assigned. Only this 2 fields will be updated.
	// Both values must be relevant in the request, it is not a PATCH! Ommitting
	// considered like an empty value, but not ignored!
	a.ge.PUT("/persons/:persId", a.h_PUT_persons_persId)

	// Creates new profile and assign the profile to the person. The old one will be rewritten
	// POST /persons/:persId/profiles - o, u, sa
	a.ge.POST("/persons/:persId/profiles", a.h_POST_persons_persId_profiles)

	// Deletes the person. All faces will be removed too.
	a.ge.DELETE("/persons/:persId", a.h_DELETE_persons_persId)

	// Deletes a person faces
	a.ge.DELETE("/persons/:persId/faces", a.h_DELETE_persons_persId_faces)

	// Gets list of cameras for the orgId (right now orgId=1), which comes from
	// the authorization of the call
	a.ge.GET("/orgs/:orgId/cameras", a.h_GET_orgs_orgId_cameras)

	// Creates new camera
	a.ge.POST("/orgs/:orgId/cameras", a.h_POST_orgs_orgId_cameras)

	// Gets information about a camera
	a.ge.GET("/cameras/:camId", a.h_GET_cameras_camId)

	// Generates new secret key for the camera. We don't keep the secret key, but its
	// hash, so it is user responsibility to get the key from the response and keeps
	// it safely. If they lost, they have to regenerate.
	a.ge.POST("/cameras/:camId/newkey", a.h_POST_cameras_camId_newkey)

```

# How to authenticate
You can use basic authentication to use any particular call, but please use it for curl and testing
puroses only (all curl examples with basic). For App please use sessions

Calls can be authenticated by session number which comes with cookie or in X-Pixty-Session header
to make a new session please do POST to /sessions with JSON {"login": "super", "password": "jopa"}
if sessions is created the session ID is returned in response, header and cookie will be set. please
use it for further calls be authorized 

# Examples
// Create a new user
curl -v -H "Content-Type: application/json" -XPOST -d '{"login": "super"}' http://api.pixty.io/users

// Set new password
curl -v -H "Content-Type: application/json" -u super:oldpasswd -XPOST -d '{"password":"newpassword"}' http://localhost:8080/users/super/password

// Create new sessions
curl -v -H "Content-Type: application/json" -X POST -d '{"login": "pixtyadmin", "password": "asdf"}' https://api.pixty.io/sessions

// User asks about his own roles (or superadmin does)
curl -v -u super:superpass http://localhost:8080/users/super/userRoles

// Create new organization
curl -v -H "Content-Type: application/json" -X POST -d '{"name": "pixty"}' -u super:asdf https://api.pixty.io/orgs

// Get all user organizations the user must be authenticated
curl -v -u pixtyadmin:123 https://api.pixty.io/orgs

// Get an org by id (1)
curl -v -u pixtyAdmin:123 http://api.pixty.io/orgs/1

// assign a user role
curl -v -H "Content-Type: application/json" -XPOST -d '{"login": "pixtyadmin", "orgId": 1, "role":"orgadmin"}' -u super:123 http://localhost:8080/orgs/1/userRoles

// Create new fields (2 here )
curl -v -H "Content-Type: application/json" -X POST -d '[{"fieldName": "First Name", "fieldType": "text"}, { "fieldName": "Last Name", "fieldType": "text"}]' http://api.pixty.io/orgs/1/fields

// get list of fields
curl -u pixtyadmin:ljkjlj  http://api.pixty.io/orgs/1/fields

// create new camera
curl -v -u pixtyAdmin:123 -H "Content-Type: application/json" -XPOST -d '{"name": "Pixty Test Camera"}'  http://localhost:8080/orgs/1/cameras

// generate new secret key - will be sent once
curl -v -XPOST 'http://api.pixty.io/cameras/1/newkey'

// Create new profile and assign fields
curl -v -u pixtyadmin:123 -H "Content-Type: application/json" -X POST -d '{"AvatarUrl": "https://api.pixty.io/images/cm-1-1504823398975.png", "orgId":1, "Attributes": [{"FieldId": 1, "Value": "Dmitry"}, {"FieldId": 2, "Value": "Spasibenko"}]}' http://api.pixty.io/profiles

// get a profile
curl http://api.pixty.io/profiles/1

// assign avatar and profile for a person - camId must be provided 
curl -v -u pixtyadmin:123 -H "Content-Type: application/json" -X PUT -d '{"AvatarUrl": "https://api.pixty.io/images/cm-1-1504823398975.png", "profileId": 1, "camId": 1}' http://api.pixty.io/persons/1b1d9491-184b-43d8-89fd-235f81fbb4df

// just get the person
curl http://api.pixty.io/persons/014aa697-45ae-4cfe-bbc7-3ea6e055a386

// get person with profiles and pictures
curl 'http://api.pixty.io/persons/014aa697-45ae-4cfe-bbc7-3ea6e055a386?details=true'

// get person with pictures and profiles with field values
curl 'http://api.pixty.io/persons/014aa697-45ae-4cfe-bbc7-3ea6e055a386?details=true&meta=true'

// Get a camera (ptt) timeline
curl https://api.pixty.io/cameras/ptt/timeline

# New organization Example

// create new org
curl -v -u super:123 -H "Content-Type: application/json" -X POST -d '{"name": "switch"}' https://api.pixty.io/orgs

>>> Location: https://api.pixty.io/orgs/3

// create the org admin 
curl -v -H "Content-Type: application/json" -XPOST -d '{"login": "switchadmin"}' https://api.pixty.io/users
curl -v -H "Content-Type: application/json" -u switchadmin: -XPOST -d '{"password":"switch123"}' https://api.pixty.io/users/switchadmin/password
curl -v -H "Content-Type: application/json" -XPOST -d '{"login": "switchadmin", "orgId": 3, "role":"orgadmin"}' -u super:123 https://api.pixty.io/orgs/3/userRoles

// create data fields
curl -v -u houseadmin:123 -H "Content-Type: application/json" -X POST -d '[{"fieldName": "First Name", "fieldType": "text"}, { "fieldName": "Last Name", "fieldType": "text"}]' https://api.pixty.io/orgs/4/fields

// create new camera
curl -v -u houseAdmin:123 -H "Content-Type: application/json" -XPOST -d '{"name": "Home sweet home"}'  https://api.pixty.io/orgs/4/cameras

// generates new camera password
curl -v -u houseadmin:123 -XPOST 'http://api.pixty.io/cameras/3/newkey'
{"id":3,"name":"Home sweet home","orgId":4,"hasSecretKey":true,"secretKey":"4UC@CCRkL1"}