import React, { useMemo } from 'react';
import { handleOnLogin } from '../api';
import Typography from '@mui/material/Typography';
import Button from '@mui/material/Button';
import steamLogo from '../icons/steam_login_lg.png';
import Link from '@mui/material/Link';
import Grid from '@mui/material/Grid';
import { useCurrentUserCtx } from '../contexts/CurrentUserCtx';
import Stack from '@mui/material/Stack';
import { ContainerWithHeader } from '../component/ContainerWithHeader';
import DoDisturbIcon from '@mui/icons-material/DoDisturb';

export interface LoginFormProps {
    message?: string;
}

export const Login = ({ message }: LoginFormProps) => {
    const { currentUser } = useCurrentUserCtx();

    const loggedInUser = useMemo(() => {
        return currentUser.steam_id.isValidIndividual();
    }, [currentUser.steam_id]);

    return (
        <Grid
            container
            marginTop={3}
            justifyContent={'center'}
            alignItems={'center'}
        >
            <Grid item>
                <ContainerWithHeader
                    title={'Permission Denied'}
                    iconLeft={<DoDisturbIcon />}
                >
                    <>
                        {loggedInUser && (
                            <Typography variant={'body1'} padding={2}>
                                Insufficient sufficient permission to access
                                this page.
                            </Typography>
                        )}
                        {!loggedInUser && (
                            <>
                                <Typography
                                    variant={'body1'}
                                    padding={2}
                                    paddingBottom={0}
                                >
                                    {message ??
                                        'To access this page, please login using your steam account below.'}
                                </Typography>
                                <Stack
                                    justifyContent="center"
                                    gap={2}
                                    flexDirection="row"
                                    width={1.0}
                                    flexWrap="wrap"
                                    padding={2}
                                >
                                    <Button
                                        sx={{ alignSelf: 'center' }}
                                        component={Link}
                                        href={handleOnLogin(
                                            window.location.pathname
                                        )}
                                    >
                                        <img
                                            src={steamLogo}
                                            alt={'Steam Login'}
                                        />
                                    </Button>
                                </Stack>
                            </>
                        )}
                    </>
                </ContainerWithHeader>
            </Grid>
        </Grid>
    );
};
