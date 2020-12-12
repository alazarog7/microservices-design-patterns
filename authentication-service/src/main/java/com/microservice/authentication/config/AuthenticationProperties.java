package com.microservice.authentication.config;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;

@Data
@ConfigurationProperties(prefix = "configuration")
public class AuthenticationProperties {
    private String homeUrl = "http://localhost:9999/api/authenticatedUser";
}
