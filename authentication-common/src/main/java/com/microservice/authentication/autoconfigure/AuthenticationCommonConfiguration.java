package com.microservice.authentication.autoconfigure;

import com.microservice.authentication.common.model.Authentication;
import io.micrometer.core.instrument.util.StringUtils;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.boot.autoconfigure.security.oauth2.resource.JwtAccessTokenConverterConfigurer;
import org.springframework.boot.autoconfigure.security.oauth2.resource.ResourceServerProperties;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Primary;
import org.springframework.core.annotation.AnnotationAwareOrderComparator;
import org.springframework.core.io.FileSystemResource;
import org.springframework.core.io.Resource;
import org.springframework.security.core.GrantedAuthority;
import org.springframework.security.oauth2.common.DefaultOAuth2AccessToken;
import org.springframework.security.oauth2.common.OAuth2AccessToken;
import org.springframework.security.oauth2.core.oidc.user.DefaultOidcUser;
import org.springframework.security.oauth2.core.oidc.user.OidcUser;
import org.springframework.security.oauth2.provider.OAuth2Authentication;
import org.springframework.security.oauth2.provider.token.DefaultTokenServices;
import org.springframework.security.oauth2.provider.token.ResourceServerTokenServices;
import org.springframework.security.oauth2.provider.token.TokenStore;
import org.springframework.security.oauth2.provider.token.store.JwtAccessTokenConverter;
import org.springframework.security.oauth2.provider.token.store.JwtTokenStore;
import org.springframework.security.oauth2.provider.token.store.KeyStoreKeyFactory;
import org.springframework.util.CollectionUtils;

import java.util.*;

import static java.util.stream.Collectors.joining;

@EnableConfigurationProperties(AuthenticationProperties.class)
@Configuration
public class AuthenticationCommonConfiguration {

    private final ResourceServerProperties resource;

    private final List<JwtAccessTokenConverterConfigurer> configurers;

    private final AuthenticationProperties authenticationProperties;

    public AuthenticationCommonConfiguration(ResourceServerProperties resource,
                                             ObjectProvider<List<JwtAccessTokenConverterConfigurer>> configurers, AuthenticationProperties authenticationProperties) {
        this.resource = resource;
        this.configurers = configurers.getIfAvailable();
        this.authenticationProperties = authenticationProperties;
    }

    @Primary
    @Bean
    @ConditionalOnMissingBean(ResourceServerTokenServices.class)
    public DefaultTokenServices jwtTokenServices(TokenStore jwtTokenStore, JwtAccessTokenConverter jwtTokenEnhancer) {
        DefaultTokenServices defaultTokenServices = new DefaultTokenServices();
        defaultTokenServices.setTokenStore(jwtTokenStore);
        defaultTokenServices.setTokenEnhancer(jwtTokenEnhancer);
        defaultTokenServices.setSupportRefreshToken(true);
        defaultTokenServices.setAccessTokenValiditySeconds(60 * 30);
        return defaultTokenServices;
    }

    @Primary
    @Bean
    @ConditionalOnMissingBean(TokenStore.class)
    public TokenStore jwtTokenStore() {
        return new JwtTokenStore(jwtTokenEnhancer());
    }

    @Primary
    @Bean
    public JwtAccessTokenConverter jwtTokenEnhancer() {
        JwtAccessTokenConverter converter = new JwtAccessTokenConverter() {
            @Override
            public OAuth2AccessToken enhance(
                OAuth2AccessToken accessToken,
                OAuth2Authentication authentication) {
                Map<String, Object> additionalInfo = new HashMap<>();
                if (authentication.getUserAuthentication() instanceof Authentication) {
                    additionalInfo.put("name",
                        ((Authentication) authentication.getUserAuthentication().getPrincipal()).getFullName());
                    additionalInfo.put("sub", authentication.getName());
                } else if (authentication.getPrincipal() instanceof OidcUser) {
                    DefaultOidcUser oidcUser = (DefaultOidcUser) authentication.getPrincipal();
                    additionalInfo.put("name", oidcUser.getEmail());
                    additionalInfo.put("sub", oidcUser.getFullName());
                    additionalInfo.put("imageUrl", oidcUser.getPicture());
                } else {
                    additionalInfo.put("sub", authentication.getName());
                }
                additionalInfo.put("auth", authentication.getAuthorities().stream()
                    .map(GrantedAuthority::getAuthority)
                    .collect(joining(",")));
                additionalInfo.put("type", "access");
                additionalInfo.put("fresh", true);
                long currentTime = new Date().getTime() / 1000;
                additionalInfo.put("iat", currentTime);
                additionalInfo.put("nbf", currentTime);
                additionalInfo.put("iss", authenticationProperties.getIssuer());
                additionalInfo.put("aud", authenticationProperties.getAud());
                ((DefaultOAuth2AccessToken) accessToken).setAdditionalInformation(additionalInfo);
                return super.enhance(accessToken, authentication);
            }
        };
        ResourceServerProperties.Jwt jwt = this.resource.getJwt();
        String keyValue = jwt.getKeyValue();
        if (StringUtils.isNotBlank(keyValue)) {
            if (!keyValue.startsWith("-----BEGIN")) {
                converter.setSigningKey(keyValue);
            }
            converter.setVerifierKey(keyValue);
        } else if (jwt.getKeyStore() != null) {
            Resource keyStore = new FileSystemResource(jwt.getKeyStore());
            char[] keyStorePassword = jwt.getKeyStorePassword().toCharArray();
            KeyStoreKeyFactory keyStoreKeyFactory = new KeyStoreKeyFactory(keyStore, keyStorePassword);

            String keyAlias = jwt.getKeyAlias();
            char[] keyPassword = Optional.ofNullable(
                jwt.getKeyPassword())
                .map(String::toCharArray).orElse(keyStorePassword);
            converter.setKeyPair(keyStoreKeyFactory.getKeyPair(keyAlias, keyPassword));
        }
        if (!CollectionUtils.isEmpty(this.configurers)) {
            AnnotationAwareOrderComparator.sort(this.configurers);
            for (JwtAccessTokenConverterConfigurer configurer : this.configurers) {
                configurer.configure(converter);
            }
        }
        return converter;
    }

}
