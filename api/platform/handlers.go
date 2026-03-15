package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/atombasedev/atombase/tools"
)

func encodeSchemaForStorage(schema Schema) ([]byte, error) {
	return tools.EncodeSchema(schema)
}

func schemaChecksum(schema Schema) (string, []byte, error) {
	schemaJSON, err := encodeSchemaForStorage(schema)
	if err != nil {
		return "", nil, err
	}
	hash := sha256.Sum256(schemaJSON)
	return hex.EncodeToString(hash[:]), schemaJSON, nil
}

func (api *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /platform/definitions", api.handleListDefinitions)
	mux.HandleFunc("GET /platform/definitions/{name}", api.handleGetDefinition)
	mux.HandleFunc("POST /platform/definitions", api.handleCreateDefinition)
	mux.HandleFunc("POST /platform/definitions/{name}/push", api.handlePushDefinition)
	mux.HandleFunc("GET /platform/definitions/{name}/history", api.handleGetDefinitionHistory)

	mux.HandleFunc("GET /platform/databases", api.handleListDatabases)
	mux.HandleFunc("GET /platform/databases/{id}", api.handleGetDatabase)
	mux.HandleFunc("POST /platform/databases", api.handleCreateDatabase)
	mux.HandleFunc("DELETE /platform/databases/{id}", api.handleDeleteDatabase)
}

func (api *API) handleListDefinitions(w http.ResponseWriter, r *http.Request) {
	items, err := api.listDefinitions(r.Context())
	if err != nil {
		tools.RespErr(w, err)
		return
	}
	tools.RespondJSON(w, http.StatusOK, items)
}

func (api *API) handleGetDefinition(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("definition name is required"))
		return
	}
	item, err := api.getDefinition(r.Context(), name)
	if err != nil {
		tools.RespErr(w, err)
		return
	}
	tools.RespondJSON(w, http.StatusOK, item)
}

func (api *API) handleCreateDefinition(w http.ResponseWriter, r *http.Request) {
	tools.LimitBody(w, r)
	defer r.Body.Close()

	var req CreateDefinitionRequest
	if err := tools.DecodeJSON(r.Body, &req); err != nil {
		tools.RespErr(w, tools.ErrInvalidJSON)
		return
	}
	if req.Name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("name is required"))
		return
	}
	if code, msg, _ := tools.ValidateResourceName(req.Name); code != "" {
		tools.RespErr(w, tools.InvalidRequestErr(msg))
		return
	}
	if req.Type == "" {
		tools.RespErr(w, tools.InvalidRequestErr("type is required"))
		return
	}
	if len(req.Schema.Tables) == 0 {
		tools.RespErr(w, tools.InvalidRequestErr("schema must have at least one table"))
		return
	}

	item, err := api.createDefinition(r.Context(), req)
	if err != nil {
		tools.RespErr(w, err)
		return
	}
	tools.RespondJSON(w, http.StatusCreated, item)
}

func (api *API) handlePushDefinition(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("definition name is required"))
		return
	}
	tools.LimitBody(w, r)
	defer r.Body.Close()
	var req PushDefinitionRequest
	if err := tools.DecodeJSON(r.Body, &req); err != nil {
		tools.RespErr(w, tools.ErrInvalidJSON)
		return
	}
	item, err := api.pushDefinition(r.Context(), name, req)
	if err != nil {
		tools.RespErr(w, err)
		return
	}
	tools.RespondJSON(w, http.StatusOK, item)
}

func (api *API) handleGetDefinitionHistory(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		tools.RespErr(w, tools.InvalidRequestErr("definition name is required"))
		return
	}
	items, err := api.getDefinitionHistory(r.Context(), name)
	if err != nil {
		tools.RespErr(w, err)
		return
	}
	tools.RespondJSON(w, http.StatusOK, items)
}

func (api *API) handleListDatabases(w http.ResponseWriter, r *http.Request) {
	items, err := api.listDatabases(r.Context())
	if err != nil {
		tools.RespErr(w, err)
		return
	}
	tools.RespondJSON(w, http.StatusOK, items)
}

func (api *API) handleGetDatabase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		tools.RespErr(w, tools.InvalidRequestErr("database id is required"))
		return
	}
	item, err := api.getDatabase(r.Context(), id)
	if err != nil {
		tools.RespErr(w, err)
		return
	}
	tools.RespondJSON(w, http.StatusOK, item)
}

func (api *API) handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	tools.LimitBody(w, r)
	defer r.Body.Close()
	var req CreateDatabaseRequest
	if err := tools.DecodeJSON(r.Body, &req); err != nil {
		tools.RespErr(w, tools.ErrInvalidJSON)
		return
	}
	if req.ID == "" {
		tools.RespErr(w, tools.InvalidRequestErr("id is required"))
		return
	}
	if code, msg, _ := tools.ValidateResourceName(req.ID); code != "" {
		tools.RespErr(w, tools.InvalidRequestErr(msg))
		return
	}
	if req.Definition == "" {
		tools.RespErr(w, tools.InvalidRequestErr("definition is required"))
		return
	}
	item, err := api.createDatabase(r.Context(), req)
	if err != nil {
		tools.RespErr(w, err)
		return
	}
	tools.RespondJSON(w, http.StatusCreated, item)
}

func (api *API) handleDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		tools.RespErr(w, tools.InvalidRequestErr("database id is required"))
		return
	}
	if err := api.deleteDatabase(r.Context(), id); err != nil {
		tools.RespErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
