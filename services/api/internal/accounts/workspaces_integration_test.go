//go:build integration

package accounts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestWorkspaceMembershipInvitationAndTransfer(t *testing.T) {
	f := newAccountFixture(t)
	ownerCookie, owner := f.signin(f.guest(), "owner-"+randomID()+"@example.com")
	editorCookie, editor := f.signin(f.guest(), "editor-"+randomID()+"@example.com")
	outsiderCookie, outsider := f.signin(f.guest(), "outsider-"+randomID()+"@example.com")
	created := fixtureJSON(t, f.request("POST", "/api/v1/workspaces", map[string]any{"name": "Team"}, ownerCookie, nil), 201)
	wid := created["id"].(string)
	base := "/api/v1/workspaces/" + wid
	fixtureJSON(t, f.request("GET", base, nil, outsiderCookie, nil), 404)
	fixtureJSON(t, f.request("GET", base+"/members", nil, ownerCookie, nil), 200)
	invitation := fixtureJSON(t, f.request("POST", base+"/invitations", map[string]any{"email": editor["email"], "role": "editor"}, ownerCookie, nil), 201)
	token := invitation["token"].(string)
	fixtureJSON(t, f.request("POST", "/api/v1/workspaces/invitations/accept", map[string]any{"token": token}, outsiderCookie, nil), 403)
	fixtureJSON(t, f.request("POST", "/api/v1/workspaces/invitations/accept", map[string]any{"token": token}, editorCookie, nil), 200)
	fixtureJSON(t, f.request("POST", "/api/v1/workspaces/invitations/accept", map[string]any{"token": token}, editorCookie, nil), 404)
	fixtureJSON(t, f.request("POST", base+"/invitations", map[string]any{"email": "boss@example.com", "role": "administrator"}, editorCookie, nil), 403)
	fixtureJSON(t, f.request("PATCH", base+"/members/"+owner["id"].(string), map[string]any{"role": "editor"}, editorCookie, nil), 403)
	personal := fixtureJSON(t, f.request("POST", "/api/v1/configurations", map[string]any{"name": "private", "settings": Defaults()}, ownerCookie, nil), 201)
	legacyKey := fixtureJSON(t, f.request("POST", "/api/v1/keys", map[string]any{"name": "Personal integration"}, ownerCookie, nil), 201)
	fixtureJSON(t, f.request("GET", base+"/configurations", nil, editorCookie, nil), 200)
	fixtureJSON(t, f.request("POST", base+"/configurations/copy", map[string]any{"configuration_id": personal["id"]}, editorCookie, nil), 404)
	fixtureJSON(t, f.request("POST", base+"/configurations/copy", map[string]any{"configuration_id": personal["id"]}, ownerCookie, nil), 201)
	fixtureJSON(t, f.request("GET", "/api/v1/configurations/"+personal["id"].(string), nil, ownerCookie, nil), 200)
	keyRequest := httptest.NewRequest("GET", "http://127.0.0.1:8080/", nil)
	keyRequest.Header.Set("Authorization", "Bearer "+legacyKey["secret"].(string))
	if principal, err := f.s.Resolve(httptest.NewRecorder(), keyRequest, false); err != nil || principal.Kind != "key" || principal.AccountID != owner["id"] {
		t.Fatal("legacy personal key changed scope after workspace creation", err)
	}
	workspaceKey := fixtureJSON(t, f.request("POST", base+"/keys", map[string]any{"name": "Pending integration"}, ownerCookie, nil), 201)
	keyList := fixtureJSON(t, f.request("GET", base+"/keys", nil, ownerCookie, nil), 200)
	if len(keyList["items"].([]any)) != 1 || keyList["items"].([]any)[0].(map[string]any)["secret"] != nil {
		t.Fatal("workspace key list disclosed a secret")
	}
	pendingRequest := httptest.NewRequest("GET", "http://127.0.0.1:8080/", nil)
	pendingRequest.Header.Set("Authorization", "Bearer "+workspaceKey["secret"].(string))
	if _, err := f.s.Resolve(httptest.NewRecorder(), pendingRequest, false); err == nil {
		t.Fatal("pending workspace key activated without paid entitlement")
	}
	f.now = f.now.Add(61 * time.Second)
	challenge := fixtureJSON(t, f.request("POST", base+"/ownership-email/start", nil, ownerCookie, nil), 202)
	challengeID := challenge["challenge_id"].(string)
	code := f.mail.code(challengeID)
	fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/verify", map[string]any{"challenge_id": challengeID, "code": code}, ownerCookie, nil), 403)
	fixtureJSON(t, f.request("POST", base+"/ownership-email/verify", map[string]any{"challenge_id": challengeID, "code": code}, editorCookie, nil), 403)
	proof := fixtureJSON(t, f.request("POST", base+"/ownership-email/verify", map[string]any{"challenge_id": challengeID, "code": code}, ownerCookie, nil), 200)
	transfer := fixtureJSON(t, f.request("POST", base+"/ownership-transfer", map[string]any{"account_id": editor["id"], "proof_id": proof["proof_id"]}, ownerCookie, nil), 201)
	fixtureJSON(t, f.request("POST", "/api/v1/workspaces/ownership/accept", map[string]any{"token": transfer["token"]}, editorCookie, nil), 200)
	fixtureJSON(t, f.request("POST", "/api/v1/workspaces/ownership/accept", map[string]any{"token": transfer["token"]}, editorCookie, nil), 404)
	var owners int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM syntelle_accounts.memberships WHERE workspace_id=$1 AND role='owner'`, wid).Scan(&owners); err != nil || owners != 1 {
		t.Fatal("ownership invariant", err, owners)
	}
	fixtureJSON(t, f.request("GET", base+"/activity", nil, ownerCookie, nil), 200)
	f.now = f.now.Add(61 * time.Second)
	lateChallenge := fixtureJSON(t, f.request("POST", base+"/ownership-email/start", nil, editorCookie, nil), 202)
	lateProof := fixtureJSON(t, f.request("POST", base+"/ownership-email/verify", map[string]any{"challenge_id": lateChallenge["challenge_id"], "code": f.mail.code(lateChallenge["challenge_id"].(string))}, editorCookie, nil), 200)
	f.now = f.now.Add(6 * time.Minute)
	fixtureJSON(t, f.request("POST", base+"/ownership-transfer", map[string]any{"account_id": owner["id"], "proof_id": lateProof["proof_id"]}, editorCookie, nil), 403)
	secondInvite := fixtureJSON(t, f.request("POST", base+"/invitations", map[string]any{"email": outsider["email"], "role": "editor"}, editorCookie, nil), 201)
	fixtureJSON(t, f.request("POST", "/api/v1/workspaces/invitations/accept", map[string]any{"token": secondInvite["token"]}, outsiderCookie, nil), 200)
	propose := func(target string) string {
		f.now = f.now.Add(61 * time.Second)
		c := fixtureJSON(t, f.request("POST", base+"/ownership-email/start", nil, editorCookie, nil), 202)
		proof := fixtureJSON(t, f.request("POST", base+"/ownership-email/verify", map[string]any{"challenge_id": c["challenge_id"], "code": f.mail.code(c["challenge_id"].(string))}, editorCookie, nil), 200)
		return fixtureJSON(t, f.request("POST", base+"/ownership-transfer", map[string]any{"account_id": target, "proof_id": proof["proof_id"]}, editorCookie, nil), 201)["token"].(string)
	}
	firstToken := propose(owner["id"].(string))
	secondToken := propose(outsider["id"].(string))
	results := make([]int, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		results[0] = f.request("POST", "/api/v1/workspaces/ownership/accept", map[string]any{"token": firstToken}, ownerCookie, nil).Code
	}()
	go func() {
		defer wg.Done()
		results[1] = f.request("POST", "/api/v1/workspaces/ownership/accept", map[string]any{"token": secondToken}, outsiderCookie, nil).Code
	}()
	wg.Wait()
	if !((results[0] == 200 && results[1] == 403) || (results[0] == 403 && results[1] == 200)) {
		t.Fatalf("concurrent transfers: %v", results)
	}
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM syntelle_accounts.memberships WHERE workspace_id=$1 AND role='owner'`, wid).Scan(&owners); err != nil || owners != 1 {
		t.Fatal("concurrent ownership invariant", err, owners)
	}
}

func TestWorkspaceInvitationExpiry(t *testing.T) {
	f := newAccountFixture(t)
	ownerCookie, _ := f.signin(f.guest(), "expiry-owner-"+randomID()+"@example.com")
	email := "expiry-member-" + randomID() + "@example.com"
	wid := fixtureJSON(t, f.request("POST", "/api/v1/workspaces", map[string]any{"name": "Expiry"}, ownerCookie, nil), 201)["id"].(string)
	invite := fixtureJSON(t, f.request("POST", "/api/v1/workspaces/"+wid+"/invitations", map[string]any{"email": email, "role": "editor"}, ownerCookie, nil), 201)
	f.now = f.now.Add(8 * 24 * time.Hour)
	memberCookie, _ := f.signin(f.guest(), email)
	fixtureJSON(t, f.request("POST", "/api/v1/workspaces/invitations/accept", map[string]any{"token": invite["token"]}, memberCookie, nil), 404)
}

func TestWorkspaceRoleMatrixAndInvitationRevocation(t *testing.T) {
	f := newAccountFixture(t)
	ownerCookie, owner := f.signin(f.guest(), "matrix-owner-"+randomID()+"@example.com")
	adminCookie, admin := f.signin(f.guest(), "matrix-admin-"+randomID()+"@example.com")
	editorCookie, editor := f.signin(f.guest(), "matrix-editor-"+randomID()+"@example.com")
	billingCookie, billing := f.signin(f.guest(), "matrix-billing-"+randomID()+"@example.com")
	wid := fixtureJSON(t, f.request("POST", "/api/v1/workspaces", map[string]any{"name": "Matrix"}, ownerCookie, nil), 201)["id"].(string)
	other := fixtureJSON(t, f.request("POST", "/api/v1/workspaces", map[string]any{"name": "Other"}, billingCookie, nil), 201)["id"].(string)
	base := "/api/v1/workspaces/" + wid
	for _, person := range []struct {
		cookie *http.Cookie
		data   map[string]any
		role   string
	}{{adminCookie, admin, "administrator"}, {editorCookie, editor, "editor"}, {billingCookie, billing, "billing"}} {
		invite := fixtureJSON(t, f.request("POST", base+"/invitations", map[string]any{"email": person.data["email"], "role": person.role}, ownerCookie, nil), 201)
		fixtureJSON(t, f.request("POST", "/api/v1/workspaces/invitations/accept", map[string]any{"token": invite["token"]}, person.cookie, nil), 200)
	}
	for _, actor := range []struct {
		cookie *http.Cookie
		role   string
	}{{ownerCookie, "owner"}, {adminCookie, "administrator"}, {editorCookie, "editor"}, {billingCookie, "billing"}} {
		wantConfig := 403
		wantKeys := 403
		wantActivity := 403
		if actor.role != "billing" {
			wantConfig = 200
		}
		if actor.role == "owner" || actor.role == "administrator" {
			wantKeys, wantActivity = 200, 200
		}
		fixtureJSON(t, f.request("GET", base+"/configurations", nil, actor.cookie, nil), wantConfig)
		fixtureJSON(t, f.request("GET", base+"/keys", nil, actor.cookie, nil), wantKeys)
		fixtureJSON(t, f.request("GET", base+"/activity", nil, actor.cookie, nil), wantActivity)
		fixtureJSON(t, f.request("GET", "/api/v1/workspaces/"+other+"/configurations", nil, actor.cookie, nil), map[bool]int{true: 200, false: 404}[actor.role == "billing"])
	}
	fixtureJSON(t, f.request("PATCH", base+"/members/"+admin["id"].(string), map[string]any{"role": "owner"}, adminCookie, nil), 400)
	fixtureJSON(t, f.request("PATCH", base+"/members/"+editor["id"].(string), map[string]any{"role": "administrator"}, adminCookie, nil), 403)
	fixtureJSON(t, f.request("POST", base+"/invitations", map[string]any{"email": "bad@example.com", "role": "billing"}, adminCookie, nil), 403)
	revoked := fixtureJSON(t, f.request("POST", base+"/invitations", map[string]any{"email": "revoked@example.com", "role": "editor"}, adminCookie, nil), 201)
	fixtureJSON(t, f.request("DELETE", base+"/invitations/"+revoked["id"].(string), nil, adminCookie, nil), 200)
	revokedCookie, _ := f.signin(f.guest(), "revoked@example.com")
	fixtureJSON(t, f.request("POST", "/api/v1/workspaces/invitations/accept", map[string]any{"token": revoked["token"]}, revokedCookie, nil), 404)
	fixtureJSON(t, f.request("DELETE", base+"/members/"+editor["id"].(string), nil, adminCookie, nil), 200)
	fixtureJSON(t, f.request("GET", base+"/configurations", nil, editorCookie, nil), 404)
	fixtureJSON(t, f.request("DELETE", base+"/members/"+owner["id"].(string), nil, adminCookie, nil), 403)
}

func TestInternalEmailEntryAndSuspension(t *testing.T) {
	f := newAccountFixture(t)
	cookie, account := f.signin(f.guest(), "staff-"+randomID()+"@example.com")
	wid := fixtureJSON(t, f.request("POST", "/api/v1/workspaces", map[string]any{"name": "Service customer"}, cookie, nil), 201)["id"].(string)
	f.pool.Exec(context.Background(), `INSERT INTO syntelle_accounts.staff_grants(account_id,role) VALUES($1,'administrator')`, account["id"])
	fixtureJSON(t, f.request("GET", "/api/v1/admin/customers", nil, cookie, nil), 403)
	f.now = f.now.Add(61 * time.Second)
	challenge := fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/start", nil, cookie, nil), 202)
	id := challenge["challenge_id"].(string)
	fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/start", nil, cookie, nil), 429)
	otherSession, _ := f.signin(f.guest(), account["email"].(string))
	fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/verify", map[string]any{"challenge_id": id, "code": f.mail.code(id)}, otherSession, nil), 403)
	wrong := f.request("POST", "/api/v1/admin/challenge/verify", map[string]any{"challenge_id": id, "code": "000000"}, cookie, nil)
	if wrong.Code != http.StatusForbidden {
		t.Fatal("wrong email code accepted")
	}
	access := fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/verify", map[string]any{"challenge_id": id, "code": f.mail.code(id)}, cookie, nil), 200)["access_token"].(string)
	fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/verify", map[string]any{"challenge_id": id, "code": f.mail.code(id)}, cookie, nil), 403)
	header := map[string]string{"X-Syntelle-Staff-Access": access}
	fixtureJSON(t, f.request("GET", "/api/v1/admin/customers", nil, cookie, header), 200)
	fixtureJSON(t, f.request("POST", "/api/v1/admin/workspaces/"+wid+"/suspend", map[string]any{"reason": "Fixture suspension"}, cookie, header), 200)
	var auditReason, auditOutcome, auditActor string
	if err := f.pool.QueryRow(context.Background(), `SELECT reason,outcome,actor_account_id FROM syntelle_accounts.workspace_audit WHERE workspace_id=$1 AND action='workspace.suspend'`, wid).Scan(&auditReason, &auditOutcome, &auditActor); err != nil || auditReason != "Fixture suspension" || auditOutcome != "succeeded" || auditActor != account["id"] {
		t.Fatal("suspension audit missing actor, reason or outcome", err)
	}
	fixtureJSON(t, f.request("GET", "/api/v1/workspaces/"+wid, nil, cookie, nil), 200)
	fixtureJSON(t, f.request("POST", "/api/v1/workspaces/"+wid+"/keys", map[string]any{"name": "blocked"}, cookie, nil), 403)
	fixtureJSON(t, f.request("POST", "/api/v1/admin/exit", map[string]any{}, cookie, header), 200)
	fixtureJSON(t, f.request("GET", "/api/v1/admin/customers", nil, cookie, header), 403)
	f.now = f.now.Add(61 * time.Second)
	expired := fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/start", nil, cookie, nil), 202)
	f.now = f.now.Add(11 * time.Minute)
	fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/verify", map[string]any{"challenge_id": expired["challenge_id"], "code": f.mail.code(expired["challenge_id"].(string))}, cookie, nil), 403)
	f.now = f.now.Add(61 * time.Second)
	limited := fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/start", nil, cookie, nil), 202)
	for range 5 {
		fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/verify", map[string]any{"challenge_id": limited["challenge_id"], "code": "000000"}, cookie, nil), 403)
	}
	fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/verify", map[string]any{"challenge_id": limited["challenge_id"], "code": f.mail.code(limited["challenge_id"].(string))}, cookie, nil), 403)
}

func TestSupportGrantAndPrivilegeBoundary(t *testing.T) {
	f := newAccountFixture(t)
	adminCookie, admin := f.signin(f.guest(), "platform-"+randomID()+"@example.com")
	supportCookie, support := f.signin(f.guest(), "support-"+randomID()+"@example.com")
	wid := fixtureJSON(t, f.request("POST", "/api/v1/workspaces", map[string]any{"name": "Support fixture"}, adminCookie, nil), 201)["id"].(string)
	key := fixtureJSON(t, f.request("POST", "/api/v1/workspaces/"+wid+"/keys", map[string]any{"name": "Future integration"}, adminCookie, nil), 201)
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO syntelle_accounts.staff_grants(account_id,role) VALUES($1,'administrator')`, admin["id"]); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(61 * time.Second)
	entry := fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/start", nil, adminCookie, nil), 202)
	fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/verify", map[string]any{"challenge_id": entry["challenge_id"], "code": f.mail.code(entry["challenge_id"].(string))}, supportCookie, nil), 403)
	access := fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/verify", map[string]any{"challenge_id": entry["challenge_id"], "code": f.mail.code(entry["challenge_id"].(string))}, adminCookie, nil), 200)["access_token"].(string)
	header := map[string]string{"X-Syntelle-Staff-Access": access}
	f.now = f.now.Add(61 * time.Second)
	challenge := fixtureJSON(t, f.request("POST", "/api/v1/admin/staff/grant-email/start", map[string]any{"account_id": support["id"]}, adminCookie, header), 202)
	proof := fixtureJSON(t, f.request("POST", "/api/v1/admin/staff/grant-email/verify", map[string]any{"account_id": support["id"], "challenge_id": challenge["challenge_id"], "code": f.mail.code(challenge["challenge_id"].(string))}, adminCookie, header), 200)
	fixtureJSON(t, f.request("POST", "/api/v1/admin/staff/grants", map[string]any{"account_id": support["id"], "proof_id": proof["proof_id"], "reason": "Read-only support"}, adminCookie, header), 200)
	fixtureJSON(t, f.request("POST", "/api/v1/admin/staff/grants", map[string]any{"account_id": support["id"], "proof_id": proof["proof_id"], "reason": "Replay"}, adminCookie, header), 403)
	fixtureJSON(t, f.request("DELETE", "/api/v1/admin/workspaces/"+wid+"/keys/"+key["id"].(string), map[string]any{"reason": "Operator revoked"}, adminCookie, header), 200)
	f.now = f.now.Add(61 * time.Second)
	supportEntry := fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/start", nil, supportCookie, nil), 202)
	supportAccess := fixtureJSON(t, f.request("POST", "/api/v1/admin/challenge/verify", map[string]any{"challenge_id": supportEntry["challenge_id"], "code": f.mail.code(supportEntry["challenge_id"].(string))}, supportCookie, nil), 200)["access_token"].(string)
	supportHeader := map[string]string{"X-Syntelle-Staff-Access": supportAccess}
	fixtureJSON(t, f.request("GET", "/api/v1/admin/customers", nil, supportCookie, supportHeader), 200)
	fixtureJSON(t, f.request("GET", "/api/v1/admin/audit", nil, supportCookie, supportHeader), 403)
	fixtureJSON(t, f.request("POST", "/api/v1/admin/workspaces/"+wid+"/suspend", map[string]any{"reason": "Not allowed"}, supportCookie, supportHeader), 403)
	f.now = f.now.Add(61 * time.Second)
	revokeChallenge := fixtureJSON(t, f.request("POST", "/api/v1/admin/staff/grant-email/start", map[string]any{"account_id": support["id"]}, adminCookie, header), 202)
	revokeProof := fixtureJSON(t, f.request("POST", "/api/v1/admin/staff/grant-email/verify", map[string]any{"account_id": support["id"], "challenge_id": revokeChallenge["challenge_id"], "code": f.mail.code(revokeChallenge["challenge_id"].(string))}, adminCookie, header), 200)
	fixtureJSON(t, f.request("DELETE", "/api/v1/admin/staff/grants/"+support["id"].(string), map[string]any{"proof_id": revokeProof["proof_id"], "reason": "Support assignment ended"}, adminCookie, header), 200)
	fixtureJSON(t, f.request("GET", "/api/v1/admin/customers", nil, supportCookie, supportHeader), 403)
}

func TestWorkspaceAuditFailureRollsBackRoleChange(t *testing.T) {
	f := newAccountFixture(t)
	ownerCookie, _ := f.signin(f.guest(), "audit-owner-"+randomID()+"@example.com")
	memberCookie, member := f.signin(f.guest(), "audit-member-"+randomID()+"@example.com")
	wid := fixtureJSON(t, f.request("POST", "/api/v1/workspaces", map[string]any{"name": "Audit fixture"}, ownerCookie, nil), 201)["id"].(string)
	invite := fixtureJSON(t, f.request("POST", "/api/v1/workspaces/"+wid+"/invitations", map[string]any{"email": member["email"], "role": "editor"}, ownerCookie, nil), 201)
	fixtureJSON(t, f.request("POST", "/api/v1/workspaces/invitations/accept", map[string]any{"token": invite["token"]}, memberCookie, nil), 200)
	ctx := context.Background()
	_, err := f.pool.Exec(ctx, `CREATE OR REPLACE FUNCTION syntelle_accounts.reject_audit_fixture() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture audit failure'; END $$`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.pool.Exec(ctx, `CREATE TRIGGER reject_audit_fixture BEFORE INSERT ON syntelle_accounts.workspace_audit FOR EACH ROW EXECUTE FUNCTION syntelle_accounts.reject_audit_fixture()`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS reject_audit_fixture ON syntelle_accounts.workspace_audit`)
		_, _ = f.pool.Exec(context.Background(), `DROP FUNCTION IF EXISTS syntelle_accounts.reject_audit_fixture()`)
	})
	response := f.request("PATCH", "/api/v1/workspaces/"+wid+"/members/"+member["id"].(string), map[string]any{"role": "billing"}, ownerCookie, nil)
	if response.Code != 503 {
		t.Fatalf("audit failure should fail closed: %d", response.Code)
	}
	var role string
	if err = f.pool.QueryRow(ctx, `SELECT role FROM syntelle_accounts.memberships WHERE workspace_id=$1 AND account_id=$2`, wid, member["id"]).Scan(&role); err != nil || role != "editor" {
		t.Fatal("role changed without audit", role, err)
	}
}
