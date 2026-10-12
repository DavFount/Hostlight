// Presentation only: preserve the backend diagnostic and availability decisions.
export function diagnosticSummary(message:string):string {
 if (/^HTTP|^HTTPS/.test(message)) return message
 if (/connection refused|actively refused/i.test(message)) return 'The connection was refused. Check that the service is running and the port is correct.'
 if (/timed out|timeout|deadline exceeded/i.test(message)) return 'No response arrived before the timeout. Check connectivity and firewall rules.'
 if (/destination unreachable|network is unreachable|host is unreachable|no route to host|unreachable network|unreachable host/i.test(message)) return 'The destination could not be reached. Check the network connection and routing.'
 if (/TTL expired/i.test(message)) return 'The ping reached its hop limit before receiving a reply. Check the network route.'
 if (/^DNS lookup failed/.test(message)) return message
 if (/no such host|lookup .*:|getaddrinfo/i.test(message)) return 'The hostname could not be resolved. Check its spelling and your DNS connection.'
 if (/permission denied|access is denied|access denied|forbidden by its access permissions/i.test(message)) return 'The check was blocked by local permissions or security settings.'
 if (/^Windows could not open a ping handle|^Windows ping failed|^Ping could not complete|^IPv6 interface was not found/.test(message)) return 'Ping could not run. Check the local network adapter and routing.'
 if (/^(dial |read |write |connect:|socket:)/i.test(message)) return 'The network check failed. Open technical details for the reported error.'
 return message
}
