package com.microservice.user.repository;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.microservice.authentication.common.model.Authority;
import com.microservice.user.model.User;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.data.mongo.DataMongoTest;
import org.springframework.boot.test.context.TestConfiguration;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Import;
import org.springframework.security.oauth2.jwt.JwtDecoder;
import org.springframework.test.context.junit.jupiter.SpringExtension;

import java.util.Arrays;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;

@ExtendWith(SpringExtension.class)
@DataMongoTest(properties = {"configuration.initialLoad=false", "logging.level.com.microservice.person.util=debug"})
@Import(ObjectMapper.class)
class UserRepositoryTest {

    @Autowired
    UserRepository userRepository;

    @TestConfiguration
    static class MockConfiguration {

        @Bean
        public JwtDecoder jwtDecoder() {
            return mock(JwtDecoder.class);
        }
    }

    @BeforeEach
    public void setup() {
        List<Authority> authorities = Arrays.asList(new Authority("ROLE_ADMIN"));

        userRepository.saveAll(Arrays.asList(
                User.builder().fullName("Test 1").authorities(authorities).email("test@gmail.com").password("Test@1").build(),
                User.builder().fullName("Test 2").authorities(authorities).email("test_2@gmail.com").password("Test@1").build()
        ));
    }

    @AfterEach
    public void tearDown() {
        userRepository.deleteAll();
    }

    @Test
    void shouldFindByEmail() {
        User user = userRepository.findByEmail("test@gmail.com");
        assertThat(user).isNotNull();
        assertThat(user.getId()).isNotNull();
        assertThat(user.getEmail()).isEqualTo("test@gmail.com");
        assertThat(user.getEnabled()).isTrue();
    }

    @Test
    void shouldReturnNullWhenNotFindEmail() {
        User user = userRepository.findByEmail("test221@gmail.com");
        assertThat(user).isNull();
    }
}