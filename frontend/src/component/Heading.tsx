import useTheme from '@mui/material/styles/useTheme';
import { FC } from 'react';
import React from 'react';
import { tf2Fonts } from '../theme';
import Grid from '@mui/material/Grid';

interface HeadingProps {
    children: JSX.Element[] | JSX.Element | string;
    bgColor?: string;
    iconLeft?: React.ReactNode;
    iconRight?: React.ReactNode;
    align?: 'flex-start' | 'center' | 'flex-end' | 'space-between';
}

export const Heading: FC<HeadingProps> = ({
    children,
    bgColor,
    iconLeft,
    iconRight,
    align
}: HeadingProps) => {
    const theme = useTheme();
    return (
        <Grid
            container
            direction="row"
            alignItems="center"
            justifyContent={align ?? 'flex-start'}
            padding={1}
            sx={{
                backgroundColor: bgColor ?? theme.palette.primary.main,
                color: theme.palette.common.white,
                ...tf2Fonts
            }}
        >
            {iconLeft && (
                <Grid item paddingRight={1}>
                    {iconLeft}
                </Grid>
            )}
            <Grid item>{children}</Grid>
            {iconRight && (
                <Grid item paddingLeft={1}>
                    {iconRight}
                </Grid>
            )}
        </Grid>
    );
};
