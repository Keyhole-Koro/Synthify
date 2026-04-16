variable "project_id" {
  type = string
}

variable "location" {
  type = string
}

variable "name" {
  type = string
}

variable "max_concurrent_dispatches" {
  type    = number
  default = 10
}

variable "max_dispatches_per_second" {
  type    = number
  default = 5
}
