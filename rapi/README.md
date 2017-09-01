# API brief comments

Copied from api.go:

```
// The ping returns pong and URI of the ping, how we see it.
	a.ge.GET("/ping", a.h_GET_ping)

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
	// Example: curl https://api.pixty.io/cameras/:camId/timeline?limit=20&maxTime=12341234
	a.ge.GET("/cameras/:camId/timeline", a.h_GET_cameras_timeline)

	// Get an known image by its file name
	// Example: curl https://api.pixty.io/images/cm-ptt1504241500992.png
	a.ge.GET("/images/:imgName", a.h_GET_images_png_download)

	// Create new org - will be used by superadmin only
	a.ge.POST("/orgs", a.h_POST_orgs)

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

	// Creates a new profile. The call allows to provide some list of field values
	//
	// Example: curl -v -H "Content-Type: application/json" -X POST -d '{"AvatarUrl": "https://api/pixty.io/images/cm-ptt1504241567000_731_353_950_572.png", "Attributes": [{"FieldId": 1, "Value": "Dmitry"}, {"FieldId": 2, "Value": "Spasibenko"}]}' http://api.pixty.io/profiles
	a.ge.POST("/profiles", a.h_POST_profiles)

	// Gets profile by its id. Only not empty fields will be returned(!)
	a.ge.GET("/profiles/:prfId", a.h_GET_profiles_prfId)

	// Updates profile AvatarUrl and list values. All fieds will be updated like
	// provided. It is not a PATCH, if a field is not set, it is considered as
	// removed. It is SNAPSHOT UPDATE
	a.ge.PUT("/profiles/:prfId", a.h_PUT_profiles_prfId)

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

	// Gets list of cameras for the orgId (right now orgId=1), which comes from
	// the authorization of the call
	a.ge.GET("/cameras", a.h_GET_cameras)

	// Creates new camera
	a.ge.POST("/cameras", a.h_POST_cameras)

	// Gets information about a camera
	a.ge.GET("/cameras/:camId", a.h_GET_cameras_camId)

	// Checks whether the camera name is available
	a.ge.GET("/cameras/:camId/name-available", a.h_GET_cameras_camId_nameAvailable)

	// Generates new secret key for the camera. We don't keep the secret key, but its
	// hash, so it is user responsibility to get the key from the response and keeps
	// it safely. If they lost, they have to regenerate.
	a.ge.POST("/cameras/:camId/newkey", a.h_POST_cameras_camId_newkey)
	
```

# Examples
// Get list of orgs
curl -v -H "Content-Type: application/json" -X POST -d '{"name": "pixty"}' http://api.pixty.io/orgs

// Get an org by id (1)
curl http://api.pixty.io/orgs/1

// Create new fields (2 here )
curl -v -H "Content-Type: application/json" -X POST -d '[{"fieldName": "First Name", "fieldType": "text"}, { "fieldName": "Last Name", "fieldType": "text"}]' http://api.pixty.io/orgs/1/fields

// get list of fields
curl http://api.pixty.io/orgs/1/fields

// create new camera
curl -H "Content-Type: application/json" -XPOST -d '{"id": "ptt"}'  http://api.pixty.io/cameras

// get list of cameras (for orgId=1 so far...)
curl http://api.pixty.io/cameras

// generate new secret key - will be sent once
curl -v -XPOST 'http://api.pixty.io/cameras/ptt/newkey'

// Create new profile and assign fields
curl -v -H "Content-Type: application/json" -X POST -d '{"AvatarUrl": "https://api/pixty.io/images/cm-ptt1504241567000_731_353_950_572.png", "Attributes": [{"FieldId": 1, "Value": "Dmitry"}, {"FieldId": 2, "Value": "Spasibenko"}]}' http://api.pixty.io/profiles

// get a profile
curl http://api.pixty.io/profiles/1

// assign avatar and profile for a person
curl -v -H "Content-Type: application/json" -X PUT -d '{"AvatarUrl": "https://api/pixty.io/images/cm-ptt1504241567000_731_353_950_572.png", "ProfileId": 1}' http://api.pixty.io/persons/014aa697-45ae-4cfe-bbc7-3ea6e055a386

// just get the person
curl http://api.pixty.io/persons/014aa697-45ae-4cfe-bbc7-3ea6e055a386

// get person with profiles and pictures
curl 'http://api.pixty.io/persons/014aa697-45ae-4cfe-bbc7-3ea6e055a386?details=true'

// get person with pictures and profiles with field values
curl 'http://api.pixty.io/persons/014aa697-45ae-4cfe-bbc7-3ea6e055a386?details=true&meta=true'

// Get a camera (ptt) timeline
curl https://api.pixty.io/cameras/ptt/timeline