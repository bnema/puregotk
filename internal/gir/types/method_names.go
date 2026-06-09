package types

// SafeReceiverMethodName avoids generating receiver methods whose names are
// known to trip go vet's stdmethods analyzer with GTK/GIO-specific signatures.
// The C symbol is unchanged; only the exported Go method name is qualified.
func SafeReceiverMethodName(receiverName, methodName string) string {
	if !hasKnownStdMethodSignatureCollision(methodName) {
		return methodName
	}
	if methodName == "" || receiverName == "" {
		return methodName
	}
	if len(methodName) >= len(receiverName) && methodName[:len(receiverName)] == receiverName {
		return methodName
	}
	return receiverName + methodName
}

func hasKnownStdMethodSignatureCollision(name string) bool {
	switch name {
	case "ReadByte", "Seek":
		return true
	default:
		return false
	}
}
