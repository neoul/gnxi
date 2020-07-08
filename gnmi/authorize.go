package gnmi

// gNMI server authorization

// validateUser validates the user.
func authorizeUser(session *Session) error {
	return nil
	// if session.Username == "" {
	// 	return errMissingUsername
	// }
	// if session.Password == "" {
	// 	return errMissingpassword
	// }
	// // [FIXME] authorize user here
	// return errAuthFailed
}
