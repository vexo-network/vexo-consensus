output "validator_public_ips" {
  description = "Public IPs for validator hosts."
  value       = aws_instance.validator[*].public_ip
}

output "validator_private_ips" {
  description = "Private IPs for validator hosts."
  value       = aws_instance.validator[*].private_ip
}

output "p2p_addresses" {
  description = "Candidate P2P addresses for topology files."
  value       = [for instance in aws_instance.validator : "${instance.public_ip}:26656"]
}

output "rpc_addresses" {
  description = "Candidate RPC addresses for private operator access."
  value       = [for instance in aws_instance.validator : "${instance.private_ip}:26657"]
}

