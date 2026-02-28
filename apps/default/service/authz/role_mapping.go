package authz

import "github.com/pitabwire/frame/security"

// BuildAccessTuple creates a tenancy_access#member tuple for a user.
func BuildAccessTuple(tenancyPath, profileID string) security.RelationTuple {
	return security.RelationTuple{
		Object:   security.ObjectRef{Namespace: NamespaceTenancyAccess, ID: tenancyPath},
		Relation: RoleMember,
		Subject:  security.SubjectRef{Namespace: NamespaceProfile, ID: profileID},
	}
}

// BuildServiceAccessTuple creates a tenancy_access#service tuple for a service bot.
func BuildServiceAccessTuple(tenancyPath, profileID string) security.RelationTuple {
	return security.RelationTuple{
		Object:   security.ObjectRef{Namespace: NamespaceTenancyAccess, ID: tenancyPath},
		Relation: RoleService,
		Subject:  security.SubjectRef{Namespace: NamespaceProfile, ID: profileID},
	}
}

// BuildServiceInheritanceTuples creates bridge tuples that allow service bots
// (who have tenancy_access#service) to inherit functional permissions in
// service_notifications via subject sets.
func BuildServiceInheritanceTuples(tenancyPath string) []security.RelationTuple {
	// Bridge: service_notifications#service <- tenancy_access#service (subject set)
	serviceBridge := security.RelationTuple{
		Object:   security.ObjectRef{Namespace: NamespaceNotifications, ID: tenancyPath},
		Relation: RoleService,
		Subject: security.SubjectRef{
			Namespace: NamespaceTenancyAccess,
			ID:        tenancyPath,
			Relation:  RoleService,
		},
	}

	// Permission bridges: service_notifications#perm <- service_notifications#service (subject set)
	permissions := RolePermissions[RoleService]
	tuples := make([]security.RelationTuple, 0, 1+len(permissions))
	tuples = append(tuples, serviceBridge)

	for _, perm := range permissions {
		tuples = append(tuples, security.RelationTuple{
			Object:   security.ObjectRef{Namespace: NamespaceNotifications, ID: tenancyPath},
			Relation: perm,
			Subject: security.SubjectRef{
				Namespace: NamespaceNotifications,
				ID:        tenancyPath,
				Relation:  RoleService,
			},
		})
	}

	return tuples
}
