package pulserpc

func ClassifySeverity(entityType EntityType, changeType ChangeType, direction Direction, extra ...string) Severity {
	switch entityType {
	case EntityStruct:
		if changeType == ChangeRemoved && direction == DirectionClientHasMore {
			return SeverityError
		}
		if changeType == ChangeAdded && direction == DirectionClientHasLess {
			return SeverityInfo
		}

	case EntityField:
		if changeType == ChangeModified && direction == DirectionMismatch {
			return SeverityError
		}
		if changeType == ChangeRemoved && direction == DirectionClientHasMore {
			return SeverityInfo
		}
		if changeType == ChangeAdded && direction == DirectionClientHasLess {
			if len(extra) > 0 && extra[0] == "required" {
				return SeverityError
			}
			return SeverityInfo
		}
		if changeType == ChangeModified && direction == DirectionClientHasLess {
			if len(extra) > 0 && extra[0] == "made_required" {
				return SeverityWarning
			}
			if len(extra) > 0 && extra[0] == "made_optional" {
				return SeverityInfo
			}
			return SeverityInfo
		}

	case EntityMethod:
		if changeType == ChangeRemoved && direction == DirectionClientHasMore {
			return SeverityError
		}
		if changeType == ChangeAdded && direction == DirectionClientHasLess {
			return SeverityWarning
		}
		if changeType == ChangeModified && direction == DirectionMismatch {
			return SeverityError
		}

	case EntityEnum:
		if changeType == ChangeRemoved && direction == DirectionClientHasMore {
			return SeverityWarning
		}
		if changeType == ChangeAdded && direction == DirectionClientHasLess {
			return SeverityWarning
		}

	case EntityError:
		if changeType == ChangeRemoved && direction == DirectionClientHasMore {
			return SeverityInfo
		}
		if changeType == ChangeAdded && direction == DirectionClientHasLess {
			return SeverityInfo
		}

	case EntityInterface:
		if changeType == ChangeRemoved && direction == DirectionClientHasMore {
			return SeverityError
		}
		if changeType == ChangeAdded && direction == DirectionClientHasLess {
			return SeverityInfo
		}
	}

	return SeverityInfo
}
