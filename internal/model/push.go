package model

// PushSubscription represents a browser's Web Push subscription.
// p256dh and auth come directly from the browser's PushSubscriptionJSON.
type PushSubscription struct {
	UserID   string `json:"-"` // never serialized to client
	Endpoint string `json:"endpoint"`
	P256DH   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

// PushSubWithDue is used by the notification scheduler.
type PushSubWithDue struct {
	PushSubscription
	DueCount int
}
