CREATE TYPE occupancy_type AS ENUM ('owner', 'tenant');
CREATE TYPE user_role AS ENUM ('syndic', 'owner', 'tenant');

CREATE TABLE unit_occupancies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    unit_id UUID NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    occupancy_type occupancy_type NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at TIMESTAMPTZ,
    UNIQUE (user_id, unit_id, occupancy_type)
);

CREATE TABLE user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    condominium_id UUID NOT NULL REFERENCES condominiums(id) ON DELETE CASCADE,
    role user_role NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    UNIQUE (user_id, condominium_id, role)
);

-- A user cannot hold both owner and tenant in the same condominium (FR-014).
CREATE UNIQUE INDEX user_roles_one_of_owner_tenant
    ON user_roles (user_id, condominium_id)
    WHERE role IN ('owner', 'tenant') AND revoked_at IS NULL;
