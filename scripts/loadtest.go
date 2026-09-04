package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {
	base := getenv("API_URL", "http://localhost:8080/graphql")
	n := 5
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	start := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errCh <- runUser(base, i)
		}(i)
	}
	wg.Wait()
	close(errCh)
	fail := 0
	for err := range errCh {
		if err != nil {
			fmt.Println("error:", err)
			fail++
		}
	}
	fmt.Printf("%d concurrent users each started one investigation in %s (failures=%d)\n", n, time.Since(start), fail)
	fmt.Println("this tests the 5-slot worker pool, not trusted-device limits")
	fmt.Println("rerun against a live API whenever you need jury concurrency proof")
	if fail > 0 {
		os.Exit(1)
	}
}

func runUser(base string, i int) error {
	email := fmt.Sprintf("loadtest-%d-%d@azula.dev", time.Now().UnixNano(), i)
	password := "password1"
	device := fmt.Sprintf("loadtest-device-%d", i)
	reg, err := gqlErr(base, "", `mutation($e:String!,$p:String!,$d:String!){register(email:$e,password:$p,deviceId:$d,deviceName:$d){token}}`, map[string]any{"e": email, "p": password, "d": device})
	if err != nil {
		return err
	}
	token := str(reg, "register", "token")
	ws, err := gqlErr(base, token, `query{workspaces{id}}`, nil)
	if err != nil {
		return err
	}
	list := ws["workspaces"].([]any)
	if len(list) == 0 {
		return fmt.Errorf("no workspace")
	}
	wsID := list[0].(map[string]any)["id"].(string)
	proj, err := gqlErr(base, token, `mutation($w:ID!){createProject(workspaceId:$w,name:"load-sample",isSample:true){id}}`, map[string]any{"w": wsID})
	if err != nil {
		return err
	}
	projectID := str(proj, "createProject", "id")
	_, err = gqlErr(base, token, `mutation($p:ID!){startInvestigation(projectId:$p){id status}}`, map[string]any{"p": projectID})
	return err
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func str(m map[string]any, keys ...string) string {
	cur := any(m)
	for _, k := range keys {
		cur = cur.(map[string]any)[k]
	}
	if cur == nil {
		return ""
	}
	return cur.(string)
}

func gqlErr(url, token, query string, vars map[string]any) (map[string]any, error) {
	body, _ := json.Marshal(map[string]any{"query": query, "variables": vars})
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var parsed struct {
		Data   map[string]any `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode: %s", raw)
	}
	if len(parsed.Errors) > 0 {
		return nil, fmt.Errorf("%s", parsed.Errors[0].Message)
	}
	return parsed.Data, nil
}
