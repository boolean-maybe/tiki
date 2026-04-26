package store

// CurrentUserDisplay projects a ReadStore's (name, email, err) tuple to a
// single display string for consumers that only need one: ruki `user()`,
// plugin-action executors, trigger setup, pipe-create trigger setup, and the
// header "User" stat. The rule is simple: name if set, otherwise email. An
// empty return value signals "no identity resolves"; callers that build a
// ruki userFunc should treat that as "unavailable" and pass nil to the
// executor so user() produces a clean error.
//
// This helper exists so every caller converges on the same projection rule.
// Earlier duplicate sites that only projected `name` silently ignored
// email-only identity configurations, which broke `assignee = user()` in
// plugin actions and triggers even when `TIKI_IDENTITY_EMAIL` was set.
func CurrentUserDisplay(s ReadStore) (string, error) {
	name, email, err := s.GetCurrentUser()
	if err != nil {
		return "", err
	}
	if name != "" {
		return name, nil
	}
	return email, nil
}

// CurrentUserDisplayFunc returns a closure suitable for ruki.NewExecutor's
// userFunc parameter, or nil when no identity resolves. A nil return is the
// deliberate signal the executor uses to surface "user() is unavailable".
func CurrentUserDisplayFunc(s ReadStore) (func() string, error) {
	display, err := CurrentUserDisplay(s)
	if err != nil {
		return nil, err
	}
	if display == "" {
		return nil, nil
	}
	return func() string { return display }, nil
}
