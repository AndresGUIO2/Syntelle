//go:build integration

package accounts

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkspacePublicationSnapshotAndIntegrationAuthorization(t *testing.T) {
	f := newAccountFixture(t)
	ctx := context.Background()
	cookie, owner := f.signin(f.guest(), "publisher-"+randomID()+"@example.com")
	other, _ := f.signin(f.guest(), "other-publisher-"+randomID()+"@example.com")
	wid := fixtureJSON(t, f.request("POST", "/api/v1/workspaces", map[string]any{"name": "Integration team"}, cookie, nil), 201)["id"].(string)
	base := "/api/v1/workspaces/" + wid
	character := "char_" + digest(randomID())[:32]
	// Fixtures stay inside the explicitly named test database; no preparation runs.
	_, err := f.pool.Exec(ctx, `INSERT INTO syntelle_accounts.character_creations(id,account_id,request_key,name,source_sha256,source_bytes,mime_type,width,height,status,package_hash,ready_at,expires_at) VALUES($1,$2,'publication-fixture','Original', $3,$4,'image/png',32,48,'ready',$5,now(),now()+interval '1 day')`, character, owner["id"], strings.Repeat("b", 64), []byte("fixture-portrait"), strings.Repeat("c", 64))
	if err != nil {
		t.Fatal(err)
	}
	settings := Defaults()
	settings.Character = character
	settings.Instructions = "Private published instructions"
	fixtureJSON(t, f.request("POST", base+"/publications", map[string]any{"creation_id": character}, other, nil), 404)
	publication := fixtureJSON(t, f.request("POST", base+"/publications", map[string]any{"creation_id": character, "name": "Website guide", "settings": settings}, cookie, nil), 201)
	pid := publication["id"].(string)
	input := map[string]any{"name": "Website", "publication_id": pid, "allowed_origins": []string{"https://customer.example"}, "capabilities": []string{"message", "speak", "interrupt", "suspend"}, "settings": settings}
	draft := fixtureJSON(t, f.request("POST", base+"/integrations", input, cookie, nil), 201)
	iid := draft["id"].(string)
	widget := draft["widget_id"].(string)
	key := fixtureJSON(t, f.request("POST", base+"/keys", map[string]any{"name": "Website server", "integration_id": iid}, cookie, nil), 201)
	resolve := func() (Principal, error) {
		r := httptest.NewRequest("GET", "http://127.0.0.1:8080/", nil)
		r.Header.Set("Authorization", "Bearer "+key["secret"].(string))
		return f.s.Resolve(httptest.NewRecorder(), r, false)
	}
	if _, err = resolve(); err == nil {
		t.Fatal("draft integration authenticated")
	}
	fixtureJSON(t, f.request("POST", base+"/integrations/"+iid+"/publish", nil, cookie, nil), 200)
	p, err := resolve()
	if err != nil || p.AccountID != "" || p.Kind != "workspace_key" || p.WorkspaceID != wid {
		t.Fatal(p, err)
	}
	current, err := f.s.ResolveWidget(ctx, widget)
	if err != nil || current.Settings.Instructions != settings.Instructions || current.Publication.PackageHash != strings.Repeat("c", 64) {
		t.Fatal(current, err)
	}
	changed := settings
	changed.Instructions = "Draft instructions only"
	input["settings"] = changed
	fixtureJSON(t, f.request("PATCH", base+"/integrations/"+iid, input, cookie, nil), 200)
	current, err = f.s.ResolveWidget(ctx, widget)
	if err != nil || current.Settings.Instructions != settings.Instructions {
		t.Fatal("draft modified published behavior", err)
	}
	if result := f.request("DELETE", "/api/v1/characters/"+character, nil, cookie, nil); result.Code != 204 {
		t.Fatal("personal deletion failed", result.Code, result.Body.String())
	}
	portrait := f.request("GET", base+"/publications/"+pid+"/portrait", nil, cookie, nil)
	if portrait.Code != 200 || portrait.Body.String() != "fixture-portrait" {
		t.Fatal("personal deletion broke workspace portrait", portrait.Code)
	}
	if _, err = f.s.ResolveWidget(ctx, widget); err != nil {
		t.Fatal("personal deletion broke published package", err)
	}
	fixtureJSON(t, f.request("POST", base+"/integrations/"+iid+"/publish", nil, cookie, nil), 200)
	if f.s.ValidatePrincipal(ctx, p) == nil {
		t.Fatal("old published authorization remained active")
	}
	p, err = resolve()
	if err != nil {
		t.Fatal(err)
	}
	if f.s.ValidatePrincipal(ctx, p) != nil {
		t.Fatal("current integration rejected")
	}
	fixtureJSON(t, f.request("DELETE", base+"/publications/"+pid, nil, cookie, nil), 200)
	if f.s.ValidatePrincipal(ctx, p) == nil {
		t.Fatal("revoked publication remained authorized")
	}
	if _, err = resolve(); err == nil {
		t.Fatal("key bypassed publication revocation")
	}
}
