

table "spdx_file" {
    field "spdxid" {
        type = string
        unique = true
    }
    field "name" {
        type = string
    }
}

table "spdx_package" {
    field "spdxid" {
        type = string
        unique = true
    }
    field "version" {
        type = string
        unique = true
    }

    join "spdx_file" { unique = true }
}

table "cpe" {
    field "part" {
        type = string
        unique = true
    }
    field "vendor" {
        type = string
        unique = true
    }
    field "product" {
        type = string
        unique = true
    }
    field "version" {
        type = string
        unique = true
    }
    field "update" {
        type = string
        unique = true
    }
}
