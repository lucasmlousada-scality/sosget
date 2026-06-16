package zendesk

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	domain string
	email  string
	token  string
	http   *http.Client
}

type Ticket struct {
	RequesterName string
	OrgName       string
}

func New(domain, email, token string) *Client {
	return &Client{
		domain: strings.TrimRight(domain, "/"),
		email:  email,
		token:  token,
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) GetTicket(id string) (*Ticket, error) {
	var ticketResp struct {
		Ticket struct {
			RequesterID    int64  `json:"requester_id"`
			OrganizationID *int64 `json:"organization_id"`
		} `json:"ticket"`
	}
	if err := c.get(fmt.Sprintf("/api/v2/tickets/%s.json", id), &ticketResp); err != nil {
		return nil, fmt.Errorf("get ticket: %w", err)
	}

	requesterName, err := c.getUserName(ticketResp.Ticket.RequesterID)
	if err != nil {
		requesterName = fmt.Sprintf("user-%d", ticketResp.Ticket.RequesterID)
	}

	orgName := ""
	if ticketResp.Ticket.OrganizationID != nil {
		orgName, _ = c.getOrgName(*ticketResp.Ticket.OrganizationID)
	}

	return &Ticket{
		RequesterName: requesterName,
		OrgName:       orgName,
	}, nil
}

func (c *Client) getUserName(id int64) (string, error) {
	var resp struct {
		User struct {
			Name string `json:"name"`
		} `json:"user"`
	}
	if err := c.get(fmt.Sprintf("/api/v2/users/%d.json", id), &resp); err != nil {
		return "", err
	}
	return resp.User.Name, nil
}

func (c *Client) getOrgName(id int64) (string, error) {
	var resp struct {
		Organization struct {
			Name string `json:"name"`
		} `json:"organization"`
	}
	if err := c.get(fmt.Sprintf("/api/v2/organizations/%d.json", id), &resp); err != nil {
		return "", err
	}
	return resp.Organization.Name, nil
}

func (c *Client) get(path string, out any) error {
	url := c.domain + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.email+"/token", c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, path)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
